package diagnostic

// Source: src/util/memory_manager.cpp:33-72; src/util/memory_manager.h:49-71.
// Source: src/util/error_codes.h; src/util/z3_exception.h.

import (
	"os"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	utext "github.com/glycerine/g3/pkg/util/text"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// Explicit standard-stream service. Go writes emit directly; native ostream
// buffering/locale/global static initialization remains a separate obligation.
// Files are borrowed, never closed or synchronized by this owner.
type MemoryStandardStreamsConfig struct {
	Budget         *arena.Budget
	Stdout, Stderr *os.File
}
type MemoryStandardStreams struct {
	self                *MemoryStandardStreams
	budget              *arena.Budget
	stdout, StderrField *utext.NumericTextStream
	header              uint64
	closed              bool
}

func NewMemoryStandardStreams(c MemoryStandardStreamsConfig) (*MemoryStandardStreams, error) {
	n := uint64(reflect.TypeOf(MemoryStandardStreams{}).Size())
	if e := c.Budget.Reserve(n); e != nil {
		return nil, e
	}
	s := &MemoryStandardStreams{budget: c.Budget, header: n}
	s.self = s
	if c.Stdout == nil {
		c.Stdout = os.Stdout
	}
	if c.Stderr == nil {
		c.Stderr = os.Stderr
	}
	var e error
	s.stdout, e = utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: c.Budget, BorrowedFile: c.Stdout})
	if e != nil {
		c.Budget.Release(n)
		return nil, e
	}
	s.StderrField, e = utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: c.Budget, BorrowedFile: c.Stderr})
	if e != nil {
		s.stdout.Close()
		c.Budget.Release(n)
		return nil, e
	}
	return s, nil
}
func (s *MemoryStandardStreams) open() error {
	if s == nil || s.self != s {
		return utext.ErrNumericStreamOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	return nil
}
func (s *MemoryStandardStreams) Stdout() (*utext.NumericTextStream, error) {
	if e := s.open(); e != nil {
		return nil, e
	}
	return s.stdout, nil
}
func (s *MemoryStandardStreams) Stderr() (*utext.NumericTextStream, error) {
	if e := s.open(); e != nil {
		return nil, e
	}
	return s.StderrField, nil
}
func (s *MemoryStandardStreams) Close() error {
	if s == nil || s.self != s {
		return utext.ErrNumericStreamOwner
	}
	if s.closed {
		return nil
	}
	if e := s.StderrField.Close(); e != nil {
		return e
	}
	if e := s.stdout.Close(); e != nil {
		return e
	}
	s.budget.Release(s.header)
	s.closed = true
	return nil
}

// Unwrap represents the C++ out_of_memory_error -> z3_error base catch relation.
type OutOfMemoryError struct{ z3exception.Z3Error }

func (e OutOfMemoryError) Unwrap() error { return e.Z3Error }

type MemoryFatalContextConfig struct {
	Budget  *arena.Budget
	Streams *MemoryStandardStreams
}
type MemoryFatalContext struct {
	self                                       *MemoryFatalContext
	budget                                     *arena.Budget
	streams                                    *MemoryStandardStreams
	header                                     uint64
	region                                     *arena.BytesRegion
	message                                    utext.RegionCString
	literal                                    string
	exitOOM, borrowed, literalSelected, closed bool
}

func NewMemoryFatalContext(c MemoryFatalContextConfig) (*MemoryFatalContext, error) {
	if e := c.Streams.open(); e != nil {
		return nil, e
	}
	if c.Streams.budget != c.Budget {
		return nil, arena.ErrConfig
	}
	n := uint64(reflect.TypeOf(MemoryFatalContext{}).Size())
	if e := c.Budget.Reserve(n); e != nil {
		return nil, e
	}
	s := &MemoryFatalContext{budget: c.Budget, streams: c.Streams, header: n}
	s.self = s
	return s, nil
}
func (s *MemoryFatalContext) open() error {
	if s == nil || s.self != s {
		return utext.ErrNumericStreamOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	return nil
}
func (s *MemoryFatalContext) MemoryBudget() *arena.Budget {
	if s == nil || s.self != s || s.closed {
		return nil
	}
	return s.budget
}

// Source stores the flag first and retains the previous pointer on false/null.
// Pointer lifetime is deliberately not inspected until insertion.
func (s *MemoryFatalContext) ExitWhenOutOfMemory(flag bool, r *arena.BytesRegion, msg utext.RegionCString) error {
	if e := s.open(); e != nil {
		return e
	}
	s.exitOOM = flag
	if flag && msg.Present {
		s.region = r
		s.message = msg
		s.borrowed = true
		s.literalSelected = false
		s.literal = ""
	}
	return nil
}

// ExitWhenOutOfMemoryLiteral binds a static source string, as shell/main.cpp
// does for the SMT-LIB2 dispatch. A nil pointer or false flag retains the
// previous message, matching exit_when_out_of_memory's pointer assignment.
func (s *MemoryFatalContext) ExitWhenOutOfMemoryLiteral(flag bool, msg *string) error {
	if e := s.open(); e != nil {
		return e
	}
	s.exitOOM = flag
	if flag && msg != nil {
		s.literal = *msg
		s.literalSelected = true
		s.borrowed = false
		s.region = nil
		s.message = utext.RegionCString{}
	}
	return nil
}
func (s *MemoryFatalContext) Close() error {
	if s == nil || s.self != s {
		return utext.ErrNumericStreamOwner
	}
	if s.closed {
		return nil
	}
	s.budget.Release(s.header)
	s.closed = true
	return nil
}
func (s *MemoryFatalContext) HandleMemoryFailure(r *umemory.Runtime, f umemory.Failure) error {
	if e := s.open(); e != nil {
		return e
	}
	switch f.Kind {
	case umemory.FailureMemout:
		if !s.exitOOM {
			return OutOfMemoryError{z3exception.MustZ3Error(z3exception.ErrorMemout)}
		}
		out, e := s.streams.Stderr()
		if e != nil {
			return e
		}
		if s.borrowed {
			e = smallObjectCStringInto(out, s.region, s.message)
		} else if s.literalSelected {
			e = out.AppendCString(s.literal)
		} else {
			e = out.AppendCString("ERROR: out of memory")
		}
		if e != nil {
			return e
		}
		if e = out.AppendByte('\n'); e != nil {
			return e
		}
		os.Exit(101)
	case umemory.FailureAllocExceeded:
		out, e := s.streams.Stdout()
		if e != nil {
			return e
		}
		if e = out.AppendCString("Maximal allocation counts "); e != nil {
			return e
		}
		n, e := r.ConfiguredMaxAllocationCount()
		if e != nil {
			return e
		}
		if e = out.AppendInt64(n); e != nil {
			return e
		}
		if e = out.AppendCString(" have been exceeded\n"); e != nil {
			return e
		}
		os.Exit(113)
	}
	return umemory.ErrFailureTransport
}
