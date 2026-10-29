package numeral

// Source: src/util/rational.h; src/util/mpz.h:allocate.

import umemory "github.com/glycerine/g3/pkg/util/memory"

func (c *RationalServiceInvocation) allocationContext() integerAllocationInvocationContext {
	if c == nil {
		return integerAllocationInvocationContext{}
	}
	return integerAllocationInvocationContext{rationalService: c, rationalServiceInvocation: c.invocation}
}
func (c integerAllocationInvocationContext) rationalServiceFrame(z *IntegerManager) *RationalServiceInvocation {
	s := c.rationalService
	if z == nil || z.self != z || s == nil || !s.valid() || s.manager == nil || s.integer != z || s.manager.Z != z || s.service.Manager != s.manager || c.rationalServiceInvocation == 0 || c.rationalServiceInvocation != s.invocation || !s.executing || s.service.lexicalExecuting != s || z.Memory.FailureTransportActive() {
		return nil
	}
	return s
}
func (z *IntegerManager) captureRationalServiceAllocationOutcome(c integerAllocationInvocationContext, backing SourceCellBacking, out *umemory.SmallAllocatorAllocationOutcome) error {
	s := c.rationalServiceFrame(z)
	if s == nil || c.raw != nil {
		return nil
	}
	if backing.Object != (umemory.ObjectRef{}) {
		s.originalCell = backing
		if z.PendingCell == backing {
			z.PendingCell = SourceCellBacking{}
		}
	}
	if out.Pending != (umemory.Block{}) {
		s.originalPending = out.Pending
		if e := z.Allocator.TransferAllocationPending(out, &s.transferred); e != nil {
			return e
		}
		if s.numeric.Allocator[0] == out.Pending {
			s.numeric.Allocator[0] = umemory.Block{}
		}
	}
	return nil
}

func (c *RationalServiceInvocation) childSource() error {
	if c == nil {
		return nil
	}
	r := &c.manager.RawArithmetic
	if c.compareGeneration != 0 && c.compareGeneration == r.compareGeneration && r.compareEnd && r.compareCause != nil {
		return r.compareCause
	}
	if c.normGeneration != 0 && c.normGeneration == r.normGeneration && r.normEnd && r.normCause != nil {
		return r.normCause
	}
	if c.addGeneration != 0 && c.addGeneration == r.addGeneration && r.addEnd && r.addCause != nil {
		return r.addCause
	}
	if c.powerGeneration != 0 && c.powerGeneration == r.PowerGeneration && r.powerEnd && r.powerCause != nil {
		return r.powerCause
	}
	for _, r := range []*integerRawInvocationReceipt{&c.extended, &c.division, &c.gcd, &c.bitwiseNot, &c.bitwiseOr, &c.bitwiseAnd, &c.bitwiseXor} {
		if e := c.integer.RawInvocationSourceCause(r); e != nil {
			return e
		}
	}
	return nil
}
func (c integerAllocationInvocationContext) recordRationalServiceRaw(z *IntegerManager, r *integerRawInvocationReceipt) {
	if s := c.rationalServiceFrame(z); s != nil {
		r.Frame.formatContext = c
		switch r.Frame {
		case &z.BitwiseNotRaw:
			s.bitwiseNot = *r
		case &z.BitwiseOrRaw:
			s.bitwiseOr = *r
		case &z.BitwiseAndRaw:
			s.bitwiseAnd = *r
		case &z.BitwiseXorRaw:
			s.bitwiseXor = *r
		case &z.DivisionRaw:
			s.division = *r
		case &z.GcdRaw:
			s.gcd = *r
		case &z.ExtendedRaw:
			s.extended = *r
		}
	}
}

func (c *RationalServiceInvocation) cleanupRationalRaw(abort bool) error {
	r := &c.manager.RawArithmetic
	if c.compareGeneration != 0 && c.compareGeneration == r.compareGeneration && r.CompareActive && (abort || r.compareEnd) {
		for r.compareCursor < 4 {
			if e := c.integer.CleanupRawNumeral(r.compareField()); e != nil {
				return e
			}
			r.compareCursor++
		}
		r.CompareActive = false
	}
	if c.normGeneration != 0 && c.normGeneration == r.normGeneration && r.NormActive && (abort || r.normEnd) {
		if e := c.integer.CleanupRawNumeral(&r.Norm); e != nil {
			return e
		}
		r.NormActive = false
	}
	if c.addGeneration != 0 && c.addGeneration == r.addGeneration && r.AddActive && (abort || r.addEnd) {
		for r.addCursor < 2 {
			p := &r.AddOne
			if r.addCursor == 1 {
				p = &r.AddTwo
			}
			if e := c.integer.CleanupRawNumeral(p); e != nil {
				return e
			}
			r.addCursor++
		}
		r.AddActive = false
	}
	if c.powerGeneration != 0 && c.powerGeneration == r.PowerGeneration && r.PowerActive && (abort || r.powerEnd) {
		for r.powerCursor < 2 {
			p := &r.Power.Num
			if r.powerCursor == 1 {
				p = &r.Power.Den
			}
			if e := c.integer.CleanupRawNumeral(p); e != nil {
				return e
			}
			r.powerCursor++
		}
		r.PowerActive = false
	}

	return nil
}
