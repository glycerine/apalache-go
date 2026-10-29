package numeral

// Source: src/util/mpz.h:393-426 (plain RAW tmp; end-only explicit DEL).

import

// This is actual source invocation state in the charged Integer owner. Body
// failure does not reach DEL; its published temporary is explicitly unjoined.
umemory "github.com/glycerine/g3/pkg/util/memory"

type integerAddMulScope struct {
	Receipt                         integerAddMulInvocationReceipt
	parentContext                   integerAllocationInvocationContext
	invocation                      uint64
	Active, executing, ended, owned bool
	Stage                           uint8
	Tmp                             Int
	Numeric                         NumericAllocationRoots

	BodyCause, SourceCause error
}
type integerAddMulInvocationReceipt struct {
	manager    *IntegerManager
	Frame      *integerAddMulScope
	invocation uint64
}

func (m *IntegerManager) MatchesAddMulInvocation(r *integerAddMulInvocationReceipt) bool {
	return r != nil && r.manager == m && m.self == m && !m.closed && r.Frame == &m.AddMulInvocation && r.invocation != 0 && r.invocation == r.Frame.invocation && r.Frame.Active
}
func (m *IntegerManager) AddMulInvocationSourceCause(r *integerAddMulInvocationReceipt) error {
	if r != nil && r.manager == m && r.Frame == &m.AddMulInvocation && r.invocation != 0 && r.invocation == r.Frame.invocation {
		return r.Frame.SourceCause
	}
	return nil
}
func (m *IntegerManager) finishAddMulSource(s *integerAddMulScope) error {
	if s != &m.AddMulInvocation {
		return nil
	}
	if s.SourceCause != nil {
		return s.SourceCause
	}
	if !s.Active || s.Stage != 3 || s.ended || s.owned {
		return nil
	}
	_, e := SourceNumeralDel(m, &s.Tmp)
	if e != nil {
		s.SourceCause = e
		return e
	}
	s.ended = true
	if !NumericAllocationPending(&s.Numeric) {
		s.Active = false
	}
	return nil
}
func (m *IntegerManager) CleanupAddMulInvocation(r *integerAddMulInvocationReceipt, abort bool) error {
	if !m.MatchesAddMulInvocation(r) {
		return nil
	}
	s := r.Frame
	if s.executing {
		return ErrNumericPending
	}
	// Only reached end DEL may complete before older caller declarations.
	if s.Stage == 3 && !s.ended && !s.owned {
		if e := m.CleanupRawNumeral(&s.Tmp); e != nil {
			return e
		}
		s.owned = true
	}
	if !abort {
		return nil
	}
	if !s.ended && !s.owned {
		if e := m.CleanupRawNumeral(&s.Tmp); e != nil {
			return e
		}
		s.owned = true
	}
	if e := s.Numeric.Cleanup(m); e != nil {
		return e
	}
	*s = integerAddMulScope{invocation: s.invocation, Stage: s.Stage, ended: s.ended, owned: s.owned, BodyCause: s.BodyCause, SourceCause: s.SourceCause}
	return nil
}

// CleanupAddMulRaw acknowledges only the actual retained AddMul/SubMul frame.
// Source body failure does not imply that its end DEL was reached.
func (m *IntegerManager) CleanupAddMulRaw() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.AddMulInvocation
	r := integerAddMulInvocationReceipt{m, s, s.invocation}
	return m.CleanupAddMulInvocation(&r, true)
}
