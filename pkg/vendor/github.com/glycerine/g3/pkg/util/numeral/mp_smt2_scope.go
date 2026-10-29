package numeral

// Source: src/util/mpz.cpp:1412 and src/util/scoped_numeral.h:31-33.

import (
	"errors"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

var ErrSMT2DisplayPending = errors.New("mpnum: scoped SMT2 display cleanup is pending")

// A completed DEFAULT scoped numeral precedes its assignment. Its actual
// owning record and retained causes are included in the IntegerManager charge.
type smt2DisplayScope struct {
	Value                   Int
	active, guard, assigned bool
	original, cleanup       error
}

type SMT2DisplayState struct {
	Pending, Guard              bool
	OriginalCause, CleanupCause error
}

func (m *IntegerManager) SMT2DisplayState() SMT2DisplayState {
	if m == nil || m.self != m {
		return SMT2DisplayState{CleanupCause: umemory.ErrRuntimeOwner}
	}
	s := &m.Smt2Display
	return SMT2DisplayState{s.active, s.guard, s.original, s.cleanup}
}

func (m *IntegerManager) beginSMT2Display() error {
	if m.Smt2Display.active {
		return ErrSMT2DisplayPending
	}
	m.Smt2Display = smt2DisplayScope{active: true}
	return nil
}

// Fresh source target.m_ptr stays null if allocate's RHS throws. The Go
// The exact failedNullRef records that action, rather than inferring from scope state.
func (m *IntegerManager) copySMT2Temporary(a *Int) error {
	e := m.Copy(&m.Smt2Display.Value, a)
	m.Smt2Display.assigned = e == nil
	return e
}

func (m *IntegerManager) finishSMT2Display(length *uint32, result *error) {
	s := &m.Smt2Display
	s.original = *result
	n := m.Natural
	// A stopped inner completed destructor prevents the older outer destructor.
	// Failed-constructor allocations alone are independent Go abort obligations.
	var cleanup error
	if n.Formatting.CompletedField != 0 || n.CharFormatting.Completed {
		cleanup = n.Formatting.cleanupCause
		if cleanup == nil {
			cleanup = n.CharFormatting.CleanupCause
		}
		if cleanup == nil {
			cleanup = n.Formatting.originalCause
		}
		if cleanup == nil {
			cleanup = ErrSMT2DisplayPending
		}
	} else if s.Value.Cell == m.FailedNullRef {
		cleanup = m.Del(&s.Value) // source DEFAULT destructor sees null; abort Go token only
	} else {
		s.guard, cleanup = sourceNumeralDel(m, &s.Value)
	}
	if cleanup != nil {
		s.cleanup = cleanup
		*length = 0
		if *result == nil {
			*result = cleanup
		} else {
			*result = errors.Join(*result, cleanup)
		}
		return
	}
	*s = smt2DisplayScope{}
}

// CleanupSMT2Display resumes completed inner destruction, then the outer
// numeral. A guarded invalid source pointer is handled only as owning Go abort;
// no source destructor event, counter rollback or pointer repair is invented.
func (m *IntegerManager) CleanupSMT2Display() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.Smt2Display
	if !s.active {
		return nil
	}
	n := m.Natural
	if n.Formatting.CompletedField != 0 {
		if e := n.unwindFormattingLocals(); e != nil {
			s.cleanup = e
			return e
		}
		n.Formatting.Active = false
		n.Formatting.PendingField = true
	}
	if e := n.destroyIntegerCharacters(); e != nil {
		s.cleanup = e
		return e
	}
	var e error
	if s.guard {
		e = m.Del(&s.Value)
	} else {
		s.guard, e = sourceNumeralDel(m, &s.Value)
	}
	if e != nil {
		s.cleanup = e
		return e
	}
	*s = smt2DisplayScope{}
	return nil
}
