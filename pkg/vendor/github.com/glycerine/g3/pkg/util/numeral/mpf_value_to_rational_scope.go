package numeral

// Source: src/util/mpf.cpp:1695-1711.

import

// Real lexical holders are embedded and charged in the MPF owner header.
// Source constructor credit is recorded only at the reached declarations.
umemory "github.com/glycerine/g3/pkg/util/memory"

type arbitraryFloatRationalConversionScope struct {
	outputDetachedNext                   *arbitraryFloatRationalConversionScope
	outputDetachedInvocation             uint64
	history                              *arbitraryFloatRationalConversionHistory
	manager                              *ArbitraryFloatManager
	sourcePrevious, outputPrevious       *arbitraryFloatRationalConversionScope
	outputManager                        *RationalManager
	outputAllocation                     arbitraryFloatRationalOutputAllocation
	outputGCD                            integerRawInvocationReceipt
	outputDivision, outputExtended       integerRawInvocationReceipt
	invocation                           uint64
	copy                                 ArbitraryFloatValueCopyReceipt
	originalCell                         SourceCellBacking
	originalPending                      umemory.Block
	transferred                          umemory.SmallAllocatorPendingAllocation
	cellRetired                          bool
	numeric                              NumericAllocationRoots
	numericPhase                         uint8
	a                                    ArbitraryFloat
	n, d                                 Int
	constructed, sourceCursor, ownCursor uint8
	active, executing, finishing         bool
	bodyCause, sourceCause, ownCause     error
}

func (m *ArbitraryFloatManager) finishRationalConversionSource() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.rationalConversion
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
	defer func() { s.finishing = false; s.recordHistory() }()
	if e := m.rationalConversionOutputSourceCause(s); e != nil {
		s.sourceCause = e
		return e
	}
	for s.sourceCursor < 3 {
		var v *Int
		switch s.sourceCursor {
		case 0:
			v = &s.d
		case 1:
			v = &s.n
		case 2:
			v = &s.a.significand
		}
		if _, e := SourceNumeralDel(m.integer, v); e != nil {
			s.sourceCause = e
			return e
		}
		s.sourceCursor++
	}
	copyPending, e := m.rationalConversionCopyPending(s)
	if e != nil {
		s.sourceCause = e
		return e
	}
	s.active = m.rationalConversionOutputPending(s) || s.outputAllocation.numeric.Cell.Object != (umemory.ObjectRef{}) || s.outputAllocation.transferred.Block() != (umemory.Block{}) || s.outputAllocation.numeric.Allocator != ([3]umemory.Block{}) || copyPending || s.numeric.Cell.Object != (umemory.ObjectRef{}) || s.transferred.Block() != (umemory.Block{}) || s.numeric.Allocator != ([3]umemory.Block{})
	return nil
}

// CleanupRationalConversion acknowledges actual lexical locals. Direct
// allocation/RAW child transport is added separately at the reached leaves;
// this routine acquires no authority from allocator directories or counts.
func (m *ArbitraryFloatManager) CleanupRationalConversion() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.rationalConversion
	if s.manager == nil {
		return nil
	}
	if s.executing || s.finishing || m.valueExecuting || m.memory.FailureTransportActive() {
		return ErrNumericPending
	}
	s.finishing = true
	s.enterLifecycle()
	defer s.leaveLifecycle()
	defer func() { s.finishing = false; s.recordHistory() }()
	if e := m.cleanupRationalConversionOutput(s, false); e != nil {
		if s.ownCause == nil {
			s.ownCause = e
		}
		return e
	}
	if e := m.CleanupValueCopy(&s.copy, false); e != nil {
		return e
	}
	for s.ownCursor < 3 {
		var v *Int
		switch s.ownCursor {
		case 0:
			v = &s.d
		case 1:
			v = &s.n
		case 2:
			v = &s.a.significand
		}
		if e := m.integer.CleanupRawNumeral(v); e != nil {
			if s.ownCause == nil {
				s.ownCause = e
			}
			return e
		}
		s.ownCursor++
	}
	if e := m.CleanupValueCopy(&s.copy, true); e != nil {
		if s.ownCause == nil {
			s.ownCause = e
		}
		return e
	}
	if e := m.cleanupRationalConversionOutput(s, true); e != nil {
		if s.ownCause == nil {
			s.ownCause = e
		}
		return e
	}
	if e := s.outputManager.Z.cleanupFormatAllocation(&s.outputAllocation.numeric, &s.outputAllocation.transferred, &s.outputAllocation.cellRetired); e != nil {
		if s.ownCause == nil {
			s.ownCause = e
		}
		return e
	}
	if e := m.integer.cleanupFormatAllocation(&s.numeric, &s.transferred, &s.cellRetired); e != nil {
		if s.ownCause == nil {
			s.ownCause = e
		}
		return e
	}
	s.unlinkDetachedOutput()
	s.active = false
	return nil
}

// A completed lexical NUM suffix does not acknowledge a failed SET allocation.
// This query follows only the issued child, never a manager-wide inventory.
func (m *ArbitraryFloatManager) rationalConversionCopyPending(s *arbitraryFloatRationalConversionScope) (bool, error) {
	if m == nil || m.self != m || s != &m.rationalConversion || s.manager != m || s.invocation == 0 {
		return false, umemory.ErrRuntimeOwner
	}
	if s.copy == (ArbitraryFloatValueCopyReceipt{}) {
		return false, nil
	}
	q, e := m.valueCopyCall(&s.copy)
	if e != nil {
		return false, e
	}
	if q.Retired {
		return false, nil
	}
	child := &q.Issued.Child
	if child.manager != m.integer || !m.integer.validCopyAllocationScope(child.Frame) || child.invocation == 0 || child.Frame.invocation != child.invocation {
		return false, umemory.ErrRuntimeOwner
	}
	return child.Frame.active, nil
}

type arbitraryFloatRationalOutputAllocation struct {
	numeric         NumericAllocationRoots
	originalCell    SourceCellBacking
	originalPending umemory.Block
	transferred     umemory.SmallAllocatorPendingAllocation
	cellRetired     bool
}

func (m *ArbitraryFloatManager) rationalConversionOutputSourceCause(s *arbitraryFloatRationalConversionScope) error {
	if !m.validConversionScope(s) {
		return umemory.ErrRuntimeOwner
	}
	z := s.outputManager.Z
	if e := z.RawInvocationSourceCause(&s.outputExtended); e != nil {
		return e
	}
	if e := z.RawInvocationSourceCause(&s.outputDivision); e != nil {
		return e
	}
	return z.RawInvocationSourceCause(&s.outputGCD)
}
func (m *ArbitraryFloatManager) rationalConversionOutputPending(s *arbitraryFloatRationalConversionScope) bool {
	if !m.validConversionScope(s) {
		return false
	}
	z := s.outputManager.Z
	return z.MatchesRawInvocation(&s.outputExtended) || z.MatchesRawInvocation(&s.outputDivision) || z.MatchesRawInvocation(&s.outputGCD)
}
func (m *ArbitraryFloatManager) cleanupRationalConversionOutput(s *arbitraryFloatRationalConversionScope, abort bool) error {
	if !m.validConversionScope(s) {
		return umemory.ErrRuntimeOwner
	}
	z := s.outputManager.Z
	if e := z.CleanupRawInvocation(&s.outputExtended, abort); e != nil {
		return e
	}
	if e := z.CleanupRawInvocation(&s.outputDivision, abort); e != nil {
		return e
	}
	return z.CleanupRawInvocation(&s.outputGCD, abort)
}

// Lifecycle exclusion alone: acquisition stays in the explicitly passed context.
func (s *arbitraryFloatRationalConversionScope) enterLifecycle() {
	source, output := s.manager.integer, s.outputManager.Z
	s.sourcePrevious = source.mpfConversionCaller
	source.mpfConversionCaller = s
	if output != source {
		s.outputPrevious = output.mpfConversionCaller
		output.mpfConversionCaller = s
	}
}
func (s *arbitraryFloatRationalConversionScope) leaveLifecycle() {
	source, output := s.manager.integer, s.outputManager.Z
	if source.mpfConversionCaller == s {
		source.mpfConversionCaller = s.sourcePrevious
	}
	if output != source && output.mpfConversionCaller == s {
		output.mpfConversionCaller = s.outputPrevious
	}
	s.sourcePrevious, s.outputPrevious = nil, nil
}
func (z *IntegerManager) conversionCallerExecuting() bool {
	if z == nil || z.self != z {
		return false
	}
	s := z.mpfConversionCaller
	return s != nil && s.manager != nil && s.manager.self == s.manager && s == &s.manager.rationalConversion && s.invocation != 0 && (s.executing || s.finishing) && (s.manager.integer == z || (s.outputManager != nil && s.outputManager.self == s.outputManager && s.outputManager.Z == z))
}

func (m *ArbitraryFloatManager) validConversionScope(s *arbitraryFloatRationalConversionScope) bool {
	return m != nil && m.self == m && s == &m.rationalConversion && s.manager == m && s.invocation != 0 && s.invocation == m.rationalConversionInvocation && m.integer != nil && m.integer.self == m.integer && s.outputManager != nil && s.outputManager.self == s.outputManager && s.outputManager.Z != nil && s.outputManager.Z.self == s.outputManager.Z
}

func (s *arbitraryFloatRationalConversionScope) bindDetachedOutput(z *IntegerManager) {
	if !s.manager.validConversionScope(s) || s.outputManager.Z != z || s.outputDetachedInvocation != 0 {
		return
	}
	s.outputDetachedInvocation = s.invocation
	s.outputDetachedNext = z.conversionDetachedOutputs
	z.conversionDetachedOutputs = s
}
func (s *arbitraryFloatRationalConversionScope) unlinkDetachedOutput() {
	if s.outputDetachedInvocation == 0 {
		return
	}
	z := s.outputManager.Z
	for p := &z.conversionDetachedOutputs; *p != nil; p = &(*p).outputDetachedNext {
		if *p == s {
			*p = s.outputDetachedNext
			s.outputDetachedNext = nil
			s.outputDetachedInvocation = 0
			return
		}
	}
}
