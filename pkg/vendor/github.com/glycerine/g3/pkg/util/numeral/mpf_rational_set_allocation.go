package numeral

// Source: src/util/mpf.cpp:233-300; src/util/mpz.h:156-164.

import (
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"reflect"
)

type ArbitraryFloatRationalSetReceipt struct {
	manager    *ArbitraryFloatManager
	frame      *arbitraryFloatRationalSetHistory
	invocation uint64
}
type arbitraryFloatRationalSetHistory struct {
	plainParent                                     *arbitraryFloatPlainRationalSetScope
	plainInvocation                                 uint64
	plainBodyCause, plainSourceCause, plainOwnCause error
	plainExpFinished                                bool

	issued                           ArbitraryFloatRationalSetReceipt
	gcd, division, extended          integerRawInvocationReceipt
	roundChild                       ArbitraryFloatRoundReceipt
	roundFrame                       *ArbitraryFloatRoundScope
	roundInvocation                  uint64
	Self                             *arbitraryFloatRationalSetHistory
	Manager                          *ArbitraryFloatManager
	Next                             *arbitraryFloatRationalSetHistory
	Invocation, header               uint64
	Numeric                          NumericAllocationRoots
	Transferred                      umemory.SmallAllocatorPendingAllocation
	cellRetired, owned               bool
	originalCell                     SourceCellBacking
	originalPending                  umemory.Block
	prevSourceCause                  error
	bodyCause, sourceCause, ownCause error
}

func (s *arbitraryFloatRationalSetScope) allocationContext() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{mpfRationalSet: s, mpfRationalSetInvocation: s.invocation}
}
func (c integerAllocationInvocationContext) rationalSetFrame(z *IntegerManager, executing bool) *arbitraryFloatRationalSetScope {
	s := c.mpfRationalSet
	if z == nil || z.self != z || s == nil || s.manager == nil || s.manager.self != s.manager || s.manager.integer != z || s != &s.manager.rationalSet || c.mpfRationalSetInvocation == 0 || c.mpfRationalSetInvocation != s.invocation {
		return nil
	}
	if executing && (!s.active || !s.executing || s.finishing || s.manager.closed || s.manager.sourceEnded || !s.manager.constructorComplete || !s.manager.rationalReady || z.closed || z.SourceFinalizing || z.constructionFailed || z.Memory == nil || z.Memory.FailureTransportActive()) {
		return nil
	}
	return s
}
func (c integerAllocationInvocationContext) prepareRationalSetHistory(z *IntegerManager) error {
	if c.mpfRationalSet == nil {
		return nil
	}
	s := c.rationalSetFrame(z, true)
	if s == nil {
		return umemory.ErrRuntimeOwner
	}
	if s.history != nil {
		return nil
	}
	bytes := uint64(reflect.TypeOf(arbitraryFloatRationalSetHistory{}).Size())
	if e := s.manager.budget.Reserve(bytes); e != nil {
		return e
	}
	h := &arbitraryFloatRationalSetHistory{Manager: s.manager, Next: s.manager.RationalSetHistories, Invocation: s.invocation, header: bytes}
	if p := s.plainParent; p != nil {
		if !p.matched(s.manager) || p.invocation != s.plainInvocation || !p.executing {
			s.manager.budget.Release(bytes)
			return umemory.ErrRuntimeOwner
		}
		h.plainParent, h.plainInvocation = p, p.invocation
		if p.receipt != nil {
			*p.receipt = ArbitraryFloatPlainRationalSetReceipt{s.manager, p, p.invocation, ArbitraryFloatRationalSetReceipt{s.manager, h, s.invocation}}
		}
	}
	h.Self = h
	h.issued = ArbitraryFloatRationalSetReceipt{s.manager, h, s.invocation}
	if s.receipt != nil {
		*s.receipt = h.issued
	}
	s.history = h
	s.manager.RationalSetHistories = h
	s.manager.rationalSetHistoryBytes += bytes
	return nil
}
func (z *IntegerManager) captureRationalSetAllocation(c integerAllocationInvocationContext, b SourceCellBacking, o *umemory.SmallAllocatorAllocationOutcome) error {
	s := c.rationalSetFrame(z, true)
	if s == nil || c.raw != nil {
		return nil
	}
	h := s.history
	if b.Object != (umemory.ObjectRef{}) {
		h.originalCell = b
		if z.PendingCell == b {
			z.PendingCell = SourceCellBacking{}
		}
	}
	if o.Pending != (umemory.Block{}) {
		h.originalPending = o.Pending
		if e := z.Allocator.TransferAllocationPending(o, &h.Transferred); e != nil {
			return e
		}
		if h.Numeric.Allocator[0] == o.Pending {
			h.Numeric.Allocator[0] = umemory.Block{}
		}
	}
	return nil
}
func (s *arbitraryFloatRationalSetScope) recordHistory() {
	if h := s.history; h != nil {
		h.prevSourceCause = s.prevSourceCause
		h.gcd = s.gcd
		h.division = s.division
		h.extended = s.extended
		h.roundChild = s.roundChild
		h.roundFrame = s.roundFrame
		h.roundInvocation = s.roundInvocation
		h.bodyCause = s.bodyCause
		h.sourceCause = s.sourceCause
		h.ownCause = s.ownCause
	}
}
func (c integerAllocationInvocationContext) recordRationalSetRaw(z *IntegerManager, r *integerRawInvocationReceipt) {
	s := c.rationalSetFrame(z, true)
	if s == nil {
		return
	}
	r.Frame.rationalSetContext = c
	switch r.Frame {
	case &z.GcdRaw:
		s.gcd = *r
	case &z.DivisionRaw:
		s.division = *r
	case &z.ExtendedRaw:
		s.extended = *r
	}
}
func (m *ArbitraryFloatManager) rationalSetChildrenSource(s *arbitraryFloatRationalSetScope) error {
	if e := m.rationalSetRoundSource(s); e != nil {
		return e
	}
	if s.prevSourceCause != nil {
		return s.prevSourceCause
	}

	if e := m.integer.RawInvocationSourceCause(&s.extended); e != nil {
		return e
	}
	if e := m.integer.RawInvocationSourceCause(&s.division); e != nil {
		return e
	}
	return m.integer.RawInvocationSourceCause(&s.gcd)
}
func (m *ArbitraryFloatManager) cleanupRationalSetChildren(s *arbitraryFloatRationalSetScope, abort bool) error {
	if e := m.rationalSetRoundCleanup(s, abort); e != nil {
		return e
	}

	if e := m.integer.CleanupRawInvocation(&s.extended, abort); e != nil {
		return e
	}
	if e := m.integer.CleanupRawInvocation(&s.division, abort); e != nil {
		return e
	}
	return m.integer.CleanupRawInvocation(&s.gcd, abort)
}
func (m *ArbitraryFloatManager) releaseRationalSetHistories() {
	for h := m.RationalSetHistories; h != nil; h = h.Next {
		m.budget.Release(h.header)
		h.header = 0
	}
	m.RationalSetHistories = nil
	m.rationalSetHistoryBytes = 0
	m.rationalSet.history = nil
}

func (m *ArbitraryFloatManager) rationalSetRoundSource(s *arbitraryFloatRationalSetScope) error {
	if s.roundChild.frame != nil {
		_, source, _ := m.RoundInvocationCauses(&s.roundChild)
		return source
	}
	if s.roundFrame == &m.RoundScope && s.roundInvocation != 0 && s.roundInvocation == m.RoundScope.Invocation {
		return m.RoundScope.sourceCause
	}
	return nil
}
func (m *ArbitraryFloatManager) rationalSetRoundCleanup(s *arbitraryFloatRationalSetScope, abort bool) error {
	if s.roundChild.frame != nil {
		return m.CleanupRoundWithReceipt(&s.roundChild, abort)
	}
	if s.roundFrame == &m.RoundScope && s.roundInvocation != 0 && s.roundInvocation == m.RoundScope.Invocation {
		return m.CleanupRoundWithReceipt(nil, abort)
	}
	return nil
}

func (m *ArbitraryFloatManager) rationalSetHistory(r *ArbitraryFloatRationalSetReceipt) (*arbitraryFloatRationalSetHistory, error) {
	if m == nil || m.self != m || r == nil || r.manager != m || r.frame == nil || r.invocation == 0 {
		return nil, umemory.ErrRuntimeOwner
	}
	for h := m.RationalSetHistories; h != nil; h = h.Next {
		if h == r.frame {
			if h.Self != h || h.Manager != m || h.Invocation != r.invocation || h.issued != *r {
				return nil, umemory.ErrRuntimeOwner
			}
			return h, nil
		}
	}
	return nil, umemory.ErrRuntimeOwner
}
func (m *ArbitraryFloatManager) ExponentRationalSetInvocationCauses(r *ArbitraryFloatRationalSetReceipt) (body, source, own error) {
	h, e := m.rationalSetHistory(r)
	if e != nil {
		return e, nil, nil
	}
	return h.bodyCause, h.sourceCause, h.ownCause
}

func (m *ArbitraryFloatManager) cleanupRationalSetHistory(h *arbitraryFloatRationalSetHistory, abort bool) error {
	if h == nil || h.Self != h || h.Manager != m || h.Invocation == 0 {
		return umemory.ErrRuntimeOwner
	}
	if h.owned {
		return nil
	}
	s := &m.rationalSet
	if s.executing || m.integer.Memory.FailureTransportActive() {
		return ErrNumericPending
	}
	stop := func(e error) error {
		if e != nil && h.ownCause == nil {
			h.ownCause = e
		}
		return e
	}
	round := func(own bool) error {
		if h.roundChild.frame != nil {
			return m.CleanupRoundWithReceipt(&h.roundChild, own)
		}
		if h.roundFrame == &m.RoundScope && h.roundInvocation != 0 && h.roundInvocation == m.RoundScope.Invocation {
			return m.CleanupRoundWithReceipt(nil, own)
		}
		return nil
	}
	if e := round(false); e != nil {
		return stop(e)
	}
	for _, r := range []*integerRawInvocationReceipt{&h.extended, &h.division, &h.gcd} {
		if e := m.integer.CleanupRawInvocation(r, false); e != nil {
			return stop(e)
		}
	}
	if !abort {
		return nil
	}
	if e := round(true); e != nil {
		return stop(e)
	}
	for _, r := range []*integerRawInvocationReceipt{&h.extended, &h.division, &h.gcd} {
		if e := m.integer.CleanupRawInvocation(r, true); e != nil {
			return stop(e)
		}
	}
	if e := m.integer.cleanupFormatAllocation(&h.Numeric, &h.Transferred, &h.cellRetired); e != nil {
		return stop(e)
	}
	h.owned = true
	return nil
}
