package numeral

// Source: src/util/mpf.cpp:1519-1569; src/util/mpq.cpp:156-185.

import umemory "github.com/glycerine/g3/pkg/util/memory"

func (c integerAllocationInvocationContext) formatFrame(z *IntegerManager, executing bool) *ArbitraryFloatFormatInvocation {
	s := c.mpfFormat
	if z == nil || z.self != z || s == nil || z.MpfParent == nil || !z.MpfParent.MatchesFormatFrame(s) || s.manager.integer != z || c.mpfFormatInvocation == 0 || c.mpfFormatInvocation != s.invocation {
		return nil
	}
	if executing && (!s.active || !s.executing || s.finishing || z.Memory.FailureTransportActive()) {
		return nil
	}
	if c.formatDecimal != nil && (c.formatDecimal != &s.Decimal || c.formatDecimal.Parent != s || (executing && !c.formatDecimal.executing)) {
		return nil
	}
	return s
}
func (s *ArbitraryFloatFormatInvocation) allocationContext() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{mpfFormat: s, mpfFormatInvocation: s.invocation}
}
func (c integerAllocationInvocationContext) recordFormatRaw(z *IntegerManager, r *integerRawInvocationReceipt) {
	if s := c.formatFrame(z, true); s != nil {
		r.Frame.formatContext = c
		if c.formatDecimal != nil {
			c.formatDecimal.division = *r
			return
		}
		switch r.Frame {
		case &z.GcdRaw:
			s.gcd = *r
		case &z.DivisionRaw:
			s.division = *r
		case &z.ExtendedRaw:
			s.extended = *r
		}
	}
}
func (m *ArbitraryFloatManager) formatChildSource(s *ArbitraryFloatFormatInvocation) error {
	z := m.integer
	if e := s.Decimal.sourceCause; e != nil {
		return e
	}
	if e := z.RawInvocationSourceCause(&s.extended); e != nil {
		return e
	}
	if e := z.RawInvocationSourceCause(&s.division); e != nil {
		return e
	}
	if e := z.RawInvocationSourceCause(&s.gcd); e != nil {
		return e
	}
	return nil
}
func (m *ArbitraryFloatManager) cleanupFormatChildren(s *ArbitraryFloatFormatInvocation, abort bool) error {
	z := m.integer
	if e := m.cleanupFormatDecimal(&s.Decimal, abort); e != nil {
		return e
	}
	for _, r := range []*integerRawInvocationReceipt{&s.extended, &s.division, &s.gcd} {
		if e := z.CleanupRawInvocation(r, abort); e != nil {
			return e
		}
	}

	if abort {

		return z.cleanupFormatAllocation(&s.Numeric, &s.Transferred, &s.cellRetired)
	}
	return nil
}

func (z *IntegerManager) captureFormatAllocationOutcome(c integerAllocationInvocationContext, backing SourceCellBacking, out *umemory.SmallAllocatorAllocationOutcome) error {
	s := c.formatFrame(z, true)
	if s == nil || c.raw != nil {
		return nil
	}
	originalPending := &s.originalPending
	original := &s.originalCell
	transferred := &s.Transferred
	roots := &s.Numeric
	if c.formatDecimal != nil {
		originalPending = &c.formatDecimal.originalPending
		original = &c.formatDecimal.originalCell
		transferred = &c.formatDecimal.Transferred
		roots = &c.formatDecimal.Numeric
	}
	if backing.Object != (umemory.ObjectRef{}) {
		*original = backing
		if z.PendingCell == backing {
			z.PendingCell = SourceCellBacking{}
		}
	}
	if out.Pending != (umemory.Block{}) {
		*originalPending = out.Pending
		if e := z.Allocator.TransferAllocationPending(out, transferred); e != nil {
			return e
		}
		if roots.Allocator[0] == out.Pending {
			roots.Allocator[0] = umemory.Block{}
		}
	}
	return nil
}
func (z *IntegerManager) cleanupFormatAllocation(roots *NumericAllocationRoots, transferred *umemory.SmallAllocatorPendingAllocation, retired *bool) error {
	if roots.Cell.Object != (umemory.ObjectRef{}) {
		if e := z.cleanupDetachedCopyCell(&roots.Cell); e != nil {
			return e
		}
		*retired = true
	}
	if transferred.Block() != (umemory.Block{}) {
		if e := z.Allocator.CleanupTransferredAllocation(transferred); e != nil {
			return e
		}
	}
	if roots.Allocator == ([3]umemory.Block{}) {
		return nil
	}
	return roots.Cleanup(z)
}
