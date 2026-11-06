package numeral

// Source: src/util/mpzzp.h:58-68,111-114; src/util/mpz.cpp:quot_rem_core.

// Direct transport of actual integer-cell acquisition leaves.
// Whole-parent and exhaustive prefix validation remain open.
type integerPNormalizeScope struct {
	factorHistoryContext integerAllocationInvocationContext
	owner                *IntegerModularManager
	invocation           uint64
	Stage                uint8
	active, executing    bool
	Roots                NumericAllocationRoots
}
type integerPNormalizeInvocationReceipt struct {
	Manager    *IntegerModularManager
	Frame      *integerPNormalizeScope
	Invocation uint64
}

func (m *IntegerModularManager) MatchesPNormalizeInvocation(r *integerPNormalizeInvocationReceipt) bool {
	return r != nil && r.Manager == m && r.Frame == &m.Normalization && r.Invocation != 0 && r.Invocation == r.Frame.invocation && r.Frame.active
}
func (m *IntegerModularManager) CleanupPNormalizeInvocation(r *integerPNormalizeInvocationReceipt, abort bool) error {
	if !m.MatchesPNormalizeInvocation(r) {
		return nil
	}
	s := r.Frame
	if s.executing {
		return ErrNumericPending
	}

	if !abort {
		return nil
	}

	if e := s.Roots.Cleanup(m.manager); e != nil {
		return e
	}
	*s = integerPNormalizeScope{owner: m, invocation: s.invocation}
	return nil
}

func (s *integerPNormalizeScope) pending() bool {
	return NumericAllocationPending(&s.Roots)
}
