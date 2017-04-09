// Copyright 2017 Scott Mansfield
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package nanolog

import "sync/atomic"
import "io"
import "bufio"
import "os"
import "reflect"
import "unicode/utf8"
import "fmt"

// MaxLoggers is the maximum number of different loggers that are allowed
const MaxLoggers = 10240

// The format string is a straightforward format inspired by the full fledged
// fmt.Fprintf function. The codes are unique to thsi package, so normal fmt
// documentation will not be applicable.

// THe format string is similar to fmt in that it uses the percent sign (a.k.a.
// the modulo operator) to signify the start of a format code. The reader is
// greedy, meaning that the parser will attempt to read as much as it can for a
// code before it stops. E.g. if you have a generic int in the middle of your
// format string immediately followed by the number 1 and a space ("%i1 "), the
// parser may complain saying that it encountered an invalid code. To fix this,
// use curly braces after the percent sign to surround the code: "%{i}1 ".

// Kinds and their corresponding format codes
//
// Kind         Code
// ------------------------
// Bool         b
// Int          i
// Int8         i8
// Int16        i16
// Int32        i32
// Int64        i64
// Uint         u
// Uint8        u8
// Uint16       u16
// Uint32       u32
// Uint64       u64
// Uintptr
// Float32      f32
// Float64      f64
// Complex64    c64
// Complex128   c128
// Array
// Chan
// Func
// Interface
// Map
// Ptr
// Slice
// String       s
// Struct
// UnsafePointer

// LogHandle is a simple handle to an internal logging data structure
// LogHandles are returned by the AddLogger method and used by the Log method to
// actually log data.
type LogHandle uint32

var (
	loggers       = make([]logger, MaxLoggers)
	curLoggersIdx = new(uint32)
)

type logger struct {
	// track varargs lengths and types that are needed
	kinds []reflect.Kind
}

var w *bufio.Writer = bufio.NewWriter(os.Stderr)

// SetWriter will set up efficient writing for the log to the output stream given.
// A raw IO stream is best.
func SetWriter(new io.Writer) {
	w.Flush()
	w = bufio.NewWriter(new)
}

// AddLogger initializes a logger and returns a handle for future logging
func AddLogger(fmt string) LogHandle {
	// save some kind of string format to the file
	idx := atomic.AddUint32(curLoggersIdx, 1) - 1

	loggers[idx] = parseLogLine(&fmt)

	return LogHandle(idx)
}

func parseLogLine(gold *string) logger {
	// make a copy we can destroy
	f := gold
	var kinds []reflect.Kind

	for len(*f) > 0 {
		if next(f) != '%' {
			continue
		}

		// Literal % sign
		if next(f) == '%' {
			continue
		}

		var requireBrace bool

		// Optional curly braces around format
		r := next(f)
		if r == '{' {
			requireBrace = true
			r = next(f)
		}

		// optimized parse tree
		switch r {
		case 'b':
			kinds = append(kinds, reflect.Bool)

		case 's':
			kinds = append(kinds, reflect.String)

		case 'i':
			r := peek(f)
			switch r {
			case '8':
				kinds = append(kinds, reflect.Int8)

			case '1':
				next(f)
				if next(f) != '6' {
					logpanic("Was expecting i16.", gold)
				}
				kinds = append(kinds, reflect.Int16)

			case '3':
				next(f)
				if next(f) != '2' {
					logpanic("Was expecting i32.", gold)
				}
				kinds = append(kinds, reflect.Int32)

			case '6':
				next(f)
				if next(f) != '4' {
					logpanic("Was expecting i64.", gold)
				}
				kinds = append(kinds, reflect.Int64)

			default:
				kinds = append(kinds, reflect.Int)
			}

		case 'u':
			r := peek(f)
			switch r {
			case '8':
				kinds = append(kinds, reflect.Uint8)

			case '1':
				next(f)
				if next(f) != '6' {
					logpanic("Was expecting u16.", gold)
				}
				kinds = append(kinds, reflect.Uint16)

			case '3':
				next(f)
				if next(f) != '2' {
					logpanic("Was expecting u32.", gold)
				}
				kinds = append(kinds, reflect.Uint32)

			case '6':
				next(f)
				if next(f) != '4' {
					logpanic("Was expecting u64.", gold)
				}
				kinds = append(kinds, reflect.Uint64)

			default:
				kinds = append(kinds, reflect.Uint)
			}

		case 'f':
			r := peek(f)
			switch r {
			case '3':
				next(f)
				if next(f) != '2' {
					logpanic("Was expecting f32.", gold)
				}
				kinds = append(kinds, reflect.Float32)

			case '6':
				next(f)
				if next(f) != '4' {
					logpanic("Was expecting f64.", gold)
				}
				kinds = append(kinds, reflect.Float64)

			default:
				logpanic("Expecting either f32 or f64", gold)
			}

		case 'c':
			r := peek(f)
			switch r {
			case '6':
				next(f)
				if next(f) != '4' {
					logpanic("Was expecting c64.", gold)
				}
				kinds = append(kinds, reflect.Complex64)

			case '1':
				next(f)
				if next(f) != '2' {
					logpanic("Was expecting c128.", gold)
				}
				if next(f) != '8' {
					logpanic("Was expecting c128.", gold)
				}
				kinds = append(kinds, reflect.Complex128)

			default:
				logpanic("Expecting either c64 or c128", gold)
			}
		}

		if requireBrace {
			if next(f) != '}' {
				logpanic("Missing '}' character", gold)
			}
		}
	}

	return logger{
		kinds: kinds,
	}
}

func peek(s *string) rune {
	r, _ := utf8.DecodeRuneInString(*s)

	if r == utf8.RuneError {
		panic("Malformed log string")
	}

	return r
}

func next(s *string) rune {
	r, n := utf8.DecodeRuneInString(*s)
	*s = (*s)[n:]

	if r == utf8.RuneError {
		panic("Malformed log string")
	}

	return r
}

// helper function to have consistently formatted panics and shorter code above
func logpanic(msg string, gold *string) {
	panic(fmt.Sprintf("Malformed log format string. %s.\n%s", msg, *gold))
}

// Log logs to the output stream for the logging package
func Log(handle LogHandle, args ...interface{}) error {
	l := loggers[handle]

	if len(l.kinds) != len(args) {
		panic("Args do not match log line")
	}

	for idx := range l.kinds {
		if l.kinds[idx] != reflect.ValueOf(args[idx]).Kind() {
			panic("Argument type does not match log line")
		}

		// write serialized version to writer
	}

	return nil
}
