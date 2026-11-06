package cmdexception

// Concrete cmd_context_types.h exception values; no command runtime scaffold.

import (
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/arena"

	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

type CmdException struct {
	z3exception.DefaultException
	line, pos int32
}

func NewCmdException(message string) CmdException {
	return CmdException{z3exception.NewDefaultException(message), -1, -1}
}
func NewCmdExceptionCString(message string) CmdException {
	if i := strings.IndexByte(message, 0); i >= 0 {
		message = message[:i]
	}
	return NewCmdException(message)
}
func NewCmdExceptionAt(message string, line, pos int32) CmdException {
	return CmdException{z3exception.NewDefaultException(message), line, pos}
}
func NewCmdExceptionSymbol(m *usymbol.Manager, message string, s usymbol.Symbol) (CmdException, error) {
	return NewCmdExceptionSymbolAt(m, message, s, -1, -1)
}

// The manager is borrowed during construction only. Checked spelling bytes go
// directly into one final caller-owned message; no Symbol.String allocation or
// owner mutation/charge occurs. Returned exceptions survive manager Close.
func NewCmdExceptionSymbolAt(m *usymbol.Manager, message string, s usymbol.Symbol, line, pos int32) (CmdException, error) {
	if m == nil {
		return CmdException{}, z3exception.ErrExceptionPrecondition
	}
	if _, e := m.FromToken(s.Token()); e != nil {
		return CmdException{}, e
	}
	if s.IsMarked() {
		return CmdException{}, usymbol.ErrInvalid
	}
	prefix := message
	if i := strings.IndexByte(message, 0); i >= 0 {
		prefix = message[:i]
	}
	var digits [12]byte
	var numeric []byte
	length := uint64(4)
	var view uparams.ParamsStringView
	if s.IsNumerical() {
		number, e := s.Number()
		if e != nil {
			return CmdException{}, e
		}
		numeric = strconv.AppendInt(digits[:0], int64(int32(number)), 10)
		length = uint64(2 + len(numeric))
	} else if !s.IsNull() {
		var e error
		view, e = uparams.SymbolCStringView(m, s)
		if e != nil {
			return CmdException{}, e
		}
		length, e = view.Length()
		if e != nil {
			return CmdException{}, e
		}
	}
	maxInt := uint64(^uint(0) >> 1)
	if length > maxInt || uint64(len(prefix)) > maxInt-length {
		return CmdException{}, arena.ErrBounds
	}
	var out strings.Builder
	out.Grow(len(prefix) + int(length))
	out.WriteString(prefix)
	if s.IsNumerical() {
		out.WriteString("k!")
		out.Write(numeric)
	} else if s.IsNull() {
		out.WriteString("null")
	} else {
		for i := uint64(0); i < length; i++ {
			v, e := view.Byte(i)
			if e != nil {
				return CmdException{}, e
			}
			out.WriteByte(v)
		}
	}
	return NewCmdExceptionAt(out.String(), line, pos), nil
}
func (e CmdException) HasPos() bool { return e.line >= 0 }

// The source guards these accessors with SASSERT only. Release builds still
// return the stored values when HasPos is false.
func (e CmdException) Line() (int32, error) {
	if cmdExceptionSourceDebug && !e.HasPos() {
		return 0, z3exception.ErrExceptionPrecondition
	}
	return e.line, nil
}
func (e CmdException) Pos() (int32, error) {
	if cmdExceptionSourceDebug && !e.HasPos() {
		return 0, z3exception.ErrExceptionPrecondition
	}
	return e.pos, nil
}

// StopParserException has no source message/code and is not a z3_exception. Its
// Error method supplies a Go-only transport label for this distinct signal.
type StopParserException struct{}

func (StopParserException) Error() string { return "g3: stop parser" }

// Unwrap preserves the source default_exception base catch through Go errors.As.
func (e CmdException) Unwrap() error { return e.DefaultException }
