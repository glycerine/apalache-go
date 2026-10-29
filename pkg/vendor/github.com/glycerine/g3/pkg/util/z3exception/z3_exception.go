package z3exception

// Concrete error classification and nonformatted util/z3_exception values.

import (
	"errors"
	"fmt"
	"strings"
)

type ErrorCode uint32

const (
	ErrorOK                ErrorCode = 0
	ErrorMemout            ErrorCode = 101
	ErrorTimeout           ErrorCode = 102
	ErrorParser            ErrorCode = 103
	ErrorUnsoundness       ErrorCode = 104
	ErrorIncompleteness    ErrorCode = 105
	ErrorINIFile           ErrorCode = 106
	ErrorNotImplementedYet ErrorCode = 107
	ErrorOpenFile          ErrorCode = 108
	ErrorCmdLine           ErrorCode = 109
	ErrorInternalFatal     ErrorCode = 110
	ErrorTypeCheck         ErrorCode = 111
	ErrorUnknownResult     ErrorCode = 112
	ErrorAllocExceeded     ErrorCode = 113
)

var ErrExceptionPrecondition = errors.New("g3: exception source precondition")

// Z3Exception is a cold error surface. Exceptions and immutable message storage
// are caller-owned return values, never boxed hot records or owner-budget data.
type Z3Exception interface {
	error
	Message() string
	ErrorCode() ErrorCode
	HasErrorCode() bool
}
type Z3Error struct{ code ErrorCode }

func NewZ3Error(code ErrorCode) (Z3Error, error) {
	// The source's nonzero check is SASSERT-only; release construction accepts
	// zero and then reports no error code through HasErrorCode.
	if z3ErrorSourceDebug && code == ErrorOK {
		return Z3Error{}, ErrExceptionPrecondition
	}
	return Z3Error{code}, nil
}

func MustZ3Error(code ErrorCode) Z3Error {
	errValue, err := NewZ3Error(code)
	if err != nil {
		panic(err)
	}
	return errValue
}
func (e Z3Error) Message() string {
	switch e.code {
	case ErrorMemout:
		return "out of memory"
	case ErrorTimeout:
		return "timeout"
	case ErrorParser:
		return "parser error"
	case ErrorUnsoundness:
		return "unsoundess"
	case ErrorIncompleteness:
		return "incompleteness"
	case ErrorINIFile:
		return "invalid INI file"
	case ErrorNotImplementedYet:
		return "not implemented yet"
	case ErrorOpenFile:
		return "open file"
	case ErrorCmdLine:
		return "invalid command line"
	case ErrorInternalFatal:
		return "internal error"
	case ErrorTypeCheck:
		return "type error"
	case ErrorAllocExceeded:
		return "number of configured allocations exceeded"
	default:
		return "unknown error"
	}
}
func (e Z3Error) Error() string        { return e.Message() }
func (e Z3Error) ErrorCode() ErrorCode { return e.code }
func (e Z3Error) HasErrorCode() bool   { return e.ErrorCode() != ErrorOK }

// DefaultException retains std::string bytes including embedded NUL; Message
// observes the source msg() C-string prefix. Immutable Go strings keep their
// backing live automatically; no borrowed owner is retained by an exception.
type DefaultException struct{ MessageField string }

func NewDefaultException(message string) DefaultException { return DefaultException{message} }

// NewDefaultExceptionf covers retained %s/%d/%u/%I64u constructor calls.
// The selected Linux AMD64 format2ostream profile writes at most 1023 bytes.
func NewDefaultExceptionf(format string, args ...any) DefaultException {
	format = format[:cStringLength(format)]
	// The retained datalog parser uses C unsigned-decimal verbs for 32- and
	// 64-bit line/symbol numbers. Go formats both unsigned types with %d.
	// Escaped %% must keep the following text literal.
	if strings.Contains(format, "%u") || strings.Contains(format, "%I64u") {
		var converted strings.Builder
		converted.Grow(len(format))
		for i := 0; i < len(format); i++ {
			if format[i] == '%' && i+1 < len(format) {
				if strings.HasPrefix(format[i+1:], "I64u") {
					converted.WriteString("%d")
					i += 4
					continue
				}
				switch format[i+1] {
				case '%':
					converted.WriteString("%%")
					i++
					continue
				case 'u':
					converted.WriteString("%d")
					i++
					continue
				}
			}
			converted.WriteByte(format[i])
		}
		format = converted.String()
	}
	copied := false
	for i, arg := range args {
		if value, ok := arg.(string); ok {
			if end := cStringLength(value); end < len(value) {
				if !copied {
					args = append([]any(nil), args...)
					copied = true
				}
				args[i] = value[:end]
			}
		}
	}
	message := fmt.Sprintf(format, args...)
	if len(message) > 1023 {
		message = message[:1023]
	}
	return DefaultException{message}
}
func (e DefaultException) Message() string      { return e.MessageField[:cStringLength(e.MessageField)] }
func (e DefaultException) Error() string        { return e.Message() }
func (e DefaultException) ErrorCode() ErrorCode { return ErrorOK }
func (e DefaultException) HasErrorCode() bool   { return e.ErrorCode() != ErrorOK }

func cStringLength(value string) int {
	if index := strings.IndexByte(value, 0); index >= 0 {
		return index
	}
	return len(value)
}
