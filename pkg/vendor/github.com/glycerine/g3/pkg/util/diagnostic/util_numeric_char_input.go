package diagnostic

// Source: src/util/debug.cpp:73-117, especially std::cin formatted extraction79-80.

import (
	"errors"
	"io"
	"os"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	utext "github.com/glycerine/g3/pkg/util/text"
)

var ErrNumericCharInputOwner = errors.New("g3: invalid formatted character input owner")

// NumericCharInputState holds the actual selected classic-locale input flags.
// Position counts bytes consumed from the borrowed file, including whitespace.
type NumericCharInputState struct {
	SkipWhitespace, EOF, Fail, Bad bool
	Position                       uint64
}
type NumericCharInputConfig struct {
	Budget       *arena.Budget
	BorrowedFile *os.File
	Tie          *utext.NumericTextStream
}

// NumericCharInput borrows its real file and tied output owner. The one-byte
// buffer is embedded in this charged cold header; no reader callback, hidden
// buffer, Unicode classification or independent output owner is introduced.
type NumericCharInput struct {
	self   *NumericCharInput
	budget *arena.Budget
	file   *os.File
	tie    *utext.NumericTextStream
	header uint64
	state  NumericCharInputState
	byte   [1]byte
	closed bool
}

func NewNumericCharInput(c NumericCharInputConfig) (*NumericCharInput, error) {
	if c.Budget == nil || c.Tie != nil && (c.Tie.CheckIdentity() != nil || c.Tie.Budget != c.Budget) {
		return nil, arena.ErrConfig
	}
	n := uint64(reflect.TypeOf(NumericCharInput{}).Size())
	if e := c.Budget.Reserve(n); e != nil {
		return nil, e
	}
	file := c.BorrowedFile
	if file == nil {
		file = os.Stdin
	}
	s := &NumericCharInput{budget: c.Budget, file: file, tie: c.Tie, header: n, state: NumericCharInputState{SkipWhitespace: true}}
	s.self = s
	return s, nil
}

// The debugger's source std::cin starts tied to the actual standard stdout.
// This factory borrows that exact owner; it neither clones nor owns the service.
func NewDebuggerCharInput(streams *MemoryStandardStreams, file *os.File) (*NumericCharInput, error) {
	out, e := streams.Stdout()
	if e != nil {
		return nil, e
	}
	return NewNumericCharInput(NumericCharInputConfig{Budget: streams.budget, BorrowedFile: file, Tie: out})
}
func (s *NumericCharInput) open() error {
	if s == nil || s.self != s {
		return ErrNumericCharInputOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	return nil
}
func (s *NumericCharInput) State() (NumericCharInputState, error) {
	if e := s.open(); e != nil {
		return NumericCharInputState{}, e
	}
	return s.state, nil
}
func (s *NumericCharInput) Tie() (*utext.NumericTextStream, error) {
	if e := s.open(); e != nil {
		return nil, e
	}
	return s.tie, nil
}
func (s *NumericCharInput) SetTie(out *utext.NumericTextStream) (*utext.NumericTextStream, error) {
	if e := s.open(); e != nil {
		return nil, e
	}
	if out != nil && (out.CheckIdentity() != nil || out.Budget != s.budget) {
		return nil, arena.ErrConfig
	}
	old := s.tie
	s.tie = out
	return old, nil
}
func (s *NumericCharInput) SetSkipWhitespace(on bool) error {
	if e := s.open(); e != nil {
		return e
	}
	s.state.SkipWhitespace = on
	return nil
}
func (s *NumericCharInput) Clear() error {
	if e := s.open(); e != nil {
		return e
	}
	s.state.EOF = false
	s.state.Fail = false
	s.state.Bad = false
	return nil
}
func numericClassicSpace(c byte) bool {
	return c == 9 || c == 10 || c == 11 || c == 12 || c == 13 || c == 32
}

// ExtractChar follows the selected istream sentry and char extraction. Failed
// extraction does not assign the caller's char. A preexisting failed stream
// does not reach tie, file or destination. Physical file errors become default
// nonthrowing bad/fail state; checked owner-lifetime guards remain explicit.
func (s *NumericCharInput) ExtractChar(dst *byte) (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	if s.state.Bad || s.state.Fail || s.state.EOF {
		s.state.Fail = true
		return false, nil
	}
	if s.tie != nil {
		if e := s.tie.Flush(); e != nil {
			return false, e
		}
	}
	for {
		n, e := s.file.Read(s.byte[:])
		if n > 0 {
			s.state.Position++
			c := s.byte[0]
			if s.state.SkipWhitespace && numericClassicSpace(c) {
				continue
			}
			if dst == nil {
				return false, arena.ErrBounds
			}
			*dst = c
			return true, nil
		}
		if errors.Is(e, io.EOF) {
			s.state.EOF = true
			s.state.Fail = true
			return false, nil
		}
		if e != nil {
			s.state.Bad = true
			s.state.Fail = true
			return false, nil
		}
		return false, arena.ErrBounds // selected os.File one-byte read made no progress.
	}
}
func (s *NumericCharInput) Close() error {
	if s == nil || s.self != s {
		return ErrNumericCharInputOwner
	}
	if s.closed {
		return nil
	}
	s.budget.Release(s.header)
	s.closed = true
	return nil
}
