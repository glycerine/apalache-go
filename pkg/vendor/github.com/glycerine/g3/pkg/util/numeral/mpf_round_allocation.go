package numeral

// Source: src/util/mpf.cpp:2100-2131 (shared concrete allocation/lifetime engine).
// Source: src/util/mpf.cpp:1963-2098; explicit reached Integer allocation leaves.

import (
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"reflect"
)

// Receipt history is materialized only at an actual allocating Integer leaf.
// It carries no cloned NUM and cannot acquire through a reused embedded frame.
type ArbitraryFloatRoundReceipt struct {
	manager    *ArbitraryFloatManager
	frame      *arbitraryFloatRoundHistory
	invocation uint64
}
type arbitraryFloatRoundHistory struct {
	issued                           ArbitraryFloatRoundReceipt
	owned                            bool
	self                             *arbitraryFloatRoundHistory
	manager                          *ArbitraryFloatManager
	next                             *arbitraryFloatRoundHistory
	invocation, header               uint64
	originalCell                     SourceCellBacking
	originalPending                  umemory.Block
	bodyCause, sourceCause, ownCause error
}

func (c integerAllocationInvocationContext) roundFrame(z *IntegerManager, executing bool) *ArbitraryFloatRoundScope {
	s := c.mpfRound
	invocation := c.mpfRoundInvocation
	if c.mpfRoundSqrt != nil {
		if s != nil {
			return nil
		}
		s = c.mpfRoundSqrt
		invocation = c.mpfRoundSqrtInvocation
	}
	if z == nil || z.self != z || s == nil || s.Manager == nil || s.Manager.self != s.Manager || s.Manager.integer != z || invocation == 0 || invocation != s.Invocation {
		return nil
	}
	if c.mpfRoundSqrt != nil {
		if s != &s.Manager.RoundSqrtScope {
			return nil
		}
	} else if s != &s.Manager.RoundScope {
		return nil
	}
	if executing && (!s.active || !s.executing || s.finishing || !s.Manager.valueExecuting || z.Memory.FailureTransportActive()) {
		return nil
	}
	return s
}
func (c integerAllocationInvocationContext) prepareRoundHistory(z *IntegerManager) error {
	if c.mpfRound == nil && c.mpfRoundSqrt == nil {
		return nil
	}
	s := c.roundFrame(z, true)
	if s == nil {
		return umemory.ErrRuntimeOwner
	}
	if e := s.parentFormattedSet.prepareRationalSetHistory(z); e != nil {
		return e
	}
	if e := s.parentFormattedSet.prepareFormattedSetHistory(z); e != nil {
		return e
	}
	if s.history != nil {
		return nil
	}
	bytes := uint64(reflect.TypeOf(arbitraryFloatRoundHistory{}).Size())
	if e := s.Manager.budget.Reserve(bytes); e != nil {
		return e
	}
	h := &arbitraryFloatRoundHistory{manager: s.Manager, next: s.Manager.roundHistories, invocation: s.Invocation, header: bytes}
	h.self = h
	h.issued = ArbitraryFloatRoundReceipt{s.Manager, h, s.Invocation}
	if s.receipt != nil {
		*s.receipt = h.issued
	}
	s.history = h
	s.Manager.roundHistories = h
	s.Manager.roundHistoryBytes += bytes
	return nil
}
func (z *IntegerManager) captureRoundAllocationOutcome(c integerAllocationInvocationContext, backing SourceCellBacking, out *umemory.SmallAllocatorAllocationOutcome) error {
	s := c.roundFrame(z, true)
	if s == nil {
		return nil
	}
	if backing.Object != (umemory.ObjectRef{}) {
		s.history.originalCell = backing
		if z.PendingCell == backing {
			z.PendingCell = SourceCellBacking{}
		}
	}
	if out.Pending != (umemory.Block{}) {
		s.history.originalPending = out.Pending
		if e := z.Allocator.TransferAllocationPending(out, &s.Transferred); e != nil {
			return e
		}
		if s.Numeric.Allocator[0] == out.Pending {
			s.Numeric.Allocator[0] = umemory.Block{}
		}
	}
	return nil
}
func (s *ArbitraryFloatRoundScope) recordRoundHistory() {
	if h := s.history; h != nil {
		h.bodyCause = s.bodyCause
		h.sourceCause = s.sourceCause
		h.ownCause = s.ownCause
	}
}
func (m *ArbitraryFloatManager) releaseRoundHistory() {
	for h := m.roundHistories; h != nil; h = h.next {
		m.budget.Release(h.header)
		h.header = 0
	}
	m.RoundScope.history = nil
	m.RoundScope.receipt = nil
	m.RoundSqrtScope.history = nil
	m.RoundSqrtScope.receipt = nil
	m.roundHistories = nil
	m.roundHistoryBytes = 0
}

func (m *ArbitraryFloatManager) roundHistory(r *ArbitraryFloatRoundReceipt) (*arbitraryFloatRoundHistory, error) {
	if m == nil || m.self != m || r == nil || r.manager != m || r.frame == nil || r.invocation == 0 {
		return nil, umemory.ErrRuntimeOwner
	}
	for h := m.roundHistories; h != nil; h = h.next {
		if h == r.frame {
			if h.self != h || h.manager != m || h.issued != *r {
				return nil, umemory.ErrRuntimeOwner
			}
			return h, nil
		}
	}
	return nil, umemory.ErrRuntimeOwner
}
func (m *ArbitraryFloatManager) RoundInvocationCauses(r *ArbitraryFloatRoundReceipt) (body, source, own error) {
	h, e := m.roundHistory(r)
	if e != nil {
		return e, nil, nil
	}
	return h.bodyCause, h.sourceCause, h.ownCause
}
