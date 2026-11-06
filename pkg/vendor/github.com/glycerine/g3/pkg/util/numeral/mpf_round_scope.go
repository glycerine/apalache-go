package numeral

// Source: src/util/mpf.cpp:2100-2131 (shared concrete allocation/lifetime engine).
// Source: src/util/mpf.cpp:1963-2098.

import

// Actual lexical NUM storage is reflected in the MPF owner header. Declaration
// flags change only at literal reached constructors; sticky is branch-scoped.
umemory "github.com/glycerine/g3/pkg/util/memory"

type ArbitraryFloatRoundScope struct {
	parentFormattedSet integerAllocationInvocationContext
	receipt            *ArbitraryFloatRoundReceipt
	Numeric            NumericAllocationRoots
	history            *arbitraryFloatRoundHistory
	Transferred        umemory.SmallAllocatorPendingAllocation
	cellRetired        bool

	Manager                                             *ArbitraryFloatManager
	Invocation                                          uint64
	sigma, cap, sticky                                  Int
	sigmaConstructed, capConstructed, stickyConstructed bool
	active, executing, finishing                        bool
	bodyCause, sourceCause, ownCause                    error
}

func (m *ArbitraryFloatManager) finishRoundSource() error {
	return m.finishRoundScopeSource(&m.RoundScope)
}
func (m *ArbitraryFloatManager) finishRoundSqrtSource() error {
	return m.finishRoundScopeSource(&m.RoundSqrtScope)
}
func (m *ArbitraryFloatManager) finishRoundScopeSource(s *ArbitraryFloatRoundScope) error {
	if m == nil || m.self != m || (s != &m.RoundScope && s != &m.RoundSqrtScope) {
		return umemory.ErrRuntimeOwner
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if !s.active {
		return nil
	}
	if s.executing || s.finishing {
		return ErrNumericPending
	}
	s.finishing = true
	defer func() { s.finishing = false; s.recordRoundHistory() }()

	for _, a := range []struct {
		v           *Int
		constructed *bool
	}{{&s.sticky, &s.stickyConstructed}, {&s.cap, &s.capConstructed}, {&s.sigma, &s.sigmaConstructed}} {
		if *a.constructed {
			if _, e := SourceNumeralDel(m.integer, a.v); e != nil {
				s.sourceCause = e
				return e
			}
			*a.constructed = false
		}
	}
	s.active = NumericAllocationPending(&s.Numeric) || s.Transferred.Block() != (umemory.Block{})

	if !s.active && s.history != nil {
		s.history.owned = true
	}
	return nil
}
func (m *ArbitraryFloatManager) CleanupRound() error {
	if e := m.CleanupRoundWithReceipt(nil, false); e != nil {
		return e
	}
	return m.CleanupRoundWithReceipt(nil, true)
}
func (m *ArbitraryFloatManager) CleanupRoundWithReceipt(r *ArbitraryFloatRoundReceipt, abort bool) error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	return m.cleanupRoundScope(&m.RoundScope, r, abort)
}
func (m *ArbitraryFloatManager) CleanupRoundSqrt() error {
	if e := m.CleanupRoundSqrtWithReceipt(nil, false); e != nil {
		return e
	}
	return m.CleanupRoundSqrtWithReceipt(nil, true)
}
func (m *ArbitraryFloatManager) CleanupRoundSqrtWithReceipt(r *ArbitraryFloatRoundReceipt, abort bool) error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	return m.cleanupRoundScope(&m.RoundSqrtScope, r, abort)
}
func (m *ArbitraryFloatManager) cleanupRoundScope(s *ArbitraryFloatRoundScope, r *ArbitraryFloatRoundReceipt, abort bool) error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	if s != &m.RoundScope && s != &m.RoundSqrtScope {
		return umemory.ErrRuntimeOwner
	}
	if s.executing || s.finishing || m.valueExecuting || m.memory != nil && m.memory.FailureTransportActive() {
		return ErrNumericPending
	}

	if r != nil {
		if *r == (ArbitraryFloatRoundReceipt{}) {
			return nil
		}
		h, e := m.roundHistory(r)
		if e != nil {
			return e
		}
		if h.owned {
			return nil
		}
		if s.history != h || s.Invocation != h.invocation {
			return umemory.ErrRuntimeOwner
		}
	}
	if s.Manager == nil {
		return nil
	}
	s.finishing = true
	defer func() { s.finishing = false; s.recordRoundHistory() }()

	for _, a := range []struct {
		v           *Int
		constructed *bool
	}{{&s.sticky, &s.stickyConstructed}, {&s.cap, &s.capConstructed}, {&s.sigma, &s.sigmaConstructed}} {
		if *a.constructed {
			if e := m.integer.CleanupRawNumeral(a.v); e != nil {
				if s.ownCause == nil {
					s.ownCause = e
				}
				return e
			}
			*a.constructed = false
		}
	}

	if !abort {
		return nil
	}

	if e := m.integer.cleanupFormatAllocation(&s.Numeric, &s.Transferred, &s.cellRetired); e != nil {
		if s.ownCause == nil {
			s.ownCause = e
		}
		return e
	}
	s.active = false
	if s.history != nil {
		s.history.owned = true
	}
	return nil
}
