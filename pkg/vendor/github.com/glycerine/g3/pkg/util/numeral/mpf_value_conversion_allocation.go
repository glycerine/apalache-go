package numeral

// Source: src/util/mpf.cpp:1695-1711,1922-1945; src/util/mpz.h:133-164.

import umemory "github.com/glycerine/g3/pkg/util/memory"

func (c integerAllocationInvocationContext) conversionFrame(z *IntegerManager, executing bool) *arbitraryFloatRationalConversionScope {
	s := c.mpfConversion
	if z == nil || z.self != z || s == nil || s.manager == nil || s.manager.self != s.manager || s != &s.manager.rationalConversion || c.mpfConversionInvocation == 0 || c.mpfConversionInvocation != s.invocation {
		return nil
	}
	if c.mpfConversionOutput {
		if s.outputManager == nil || s.outputManager.self != s.outputManager || s.outputManager.Z != z {
			return nil
		}
	} else if s.manager.integer != z {
		return nil
	}

	if executing && (!s.active || !s.executing || s.finishing || s.manager.closed || s.manager.sourceEnded || z.Memory.FailureTransportActive()) {
		return nil
	}
	return s
}
func (s *arbitraryFloatRationalConversionScope) allocationContext() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{mpfConversion: s, mpfConversionInvocation: s.invocation}
}

// The exact allocation outcome is recorded only at the actual failed leaf.
// This is a transfer of an issued root, never a later allocator census.
func (s *arbitraryFloatRationalConversionScope) outputAllocationContext() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{mpfConversion: s, mpfConversionInvocation: s.invocation, mpfConversionOutput: true}
}

func (z *IntegerManager) captureConversionAllocationOutcome(c integerAllocationInvocationContext, backing SourceCellBacking, out *umemory.SmallAllocatorAllocationOutcome) error {
	s := c.conversionFrame(z, true)
	if s == nil || c.raw != nil {
		return nil
	}
	original, pending, transferred, roots := &s.originalCell, &s.originalPending, &s.transferred, &s.numeric
	if c.mpfConversionOutput {
		o := &s.outputAllocation
		original, pending, transferred, roots = &o.originalCell, &o.originalPending, &o.transferred, &o.numeric
	}
	if backing.Object != (umemory.ObjectRef{}) {
		*original = backing
		if c.mpfConversionOutput {
			s.bindDetachedOutput(z)
		}
		if z.PendingCell == backing {
			z.PendingCell = SourceCellBacking{}
		}
	}
	if out.Pending != (umemory.Block{}) {
		*pending = out.Pending
		if e := z.Allocator.TransferAllocationPending(out, transferred); e != nil {
			return e
		}
		if roots.Allocator[0] == out.Pending {
			roots.Allocator[0] = umemory.Block{}
		}
	}
	return nil
}

func (c integerAllocationInvocationContext) recordConversionRaw(z *IntegerManager, r *integerRawInvocationReceipt) {
	s := c.conversionFrame(z, true)
	if s == nil || !c.mpfConversionOutput {
		return
	}
	r.Frame.conversionContext = c
	switch r.Frame {
	case &z.GcdRaw:
		s.outputGCD = *r
	case &z.DivisionRaw:
		s.outputDivision = *r
	case &z.ExtendedRaw:
		s.outputExtended = *r
	}
}
