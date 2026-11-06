package polynomial

// Source: src/math/polynomial/polynomial.cpp:862-884;
// src/util/small_object_allocator.cpp:104-145.

import (
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

type monomialAllocatorScope struct {
	owner                          *MonomialManager
	outcome                        umemory.SmallAllocatorAllocationOutcome
	invocation, creationGeneration uint64
	sourceCause                    error
}
type monomialAllocatorInvocationReceipt struct {
	manager    *MonomialManager
	frame      *monomialAllocatorScope
	outcome    *umemory.SmallAllocatorAllocationOutcome
	invocation uint64
}

func (m *MonomialManager) matchesMonomialAllocatorInvocation(r *monomialAllocatorInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.manager == m && r.frame == &m.allocatorInvocation && r.frame.owner == m && r.outcome == &r.frame.outcome && r.invocation != 0 && r.invocation == r.frame.invocation && m.allocator.AllocationOutcomeInvocation(r.outcome) == r.invocation
}
func (m *MonomialManager) cleanupMonomialAllocatorInvocation(r *monomialAllocatorInvocationReceipt, abort bool) error {
	if !m.matchesMonomialAllocatorInvocation(r) || !abort || r.outcome.Pending == (umemory.Block{}) {
		return nil
	}
	current, e := m.allocator.PendingRoots()
	if e != nil {
		return e
	}
	// The actual Allocate leaf can acquire only pending[0]. Scratch is foreign.
	if current != ([3]umemory.Block{r.outcome.Pending, {}, {}}) {
		return ErrMonomialPending
	}
	e = m.allocator.CleanupPending()
	after, x := m.allocator.PendingRoots()
	if after[0] == (umemory.Block{}) {
		r.outcome.Pending = umemory.Block{}
	}
	return polynomialNewtonJoin(e, x)
}
