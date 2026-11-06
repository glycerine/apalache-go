package numeral

// Source: src/util/mpf.cpp:108-112; src/util/mpf.h:77.

import

// SetFractionSigned follows the original scoped rational conversion. The
// signed32 numerator/denominator overload retains its reached checked domain.
umemory "github.com/glycerine/g3/pkg/util/memory"

func (m *ArbitraryFloatManager) SetFractionSigned(out *ArbitraryFloat, ebits, sbits uint32, mode ArbitraryFloatRoundingMode, n, d int32) (cause error) {
	if e := m.valueGuard(out); e != nil {
		return e
	}
	s := &m.signedFraction
	if s.active || s.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	*s = arbitraryFloatSignedFractionScope{manager: m, invocation: s.invocation + 1, q: NewRational(), qLive: true, active: true, executing: true}
	s.hooks.Policy = s
	if e := s.bind(); e != nil {
		s.executing = false
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = JoinNumericErrors(cause, m.finishSignedFractionSource())
		cause = JoinNumericErrors(cause, s.unbind())
		if cause == nil {
			s.active = false
		}
	}()
	c := integerAllocationInvocationContext{Caller: &s.hooks}
	if e := m.rational.setFractionSignedSourceWithAllocationContext(&s.q, n, d, c); e != nil {
		return e
	}
	// Completed normalization adapters must be joined before conversion reuses
	// their physical primitive frames. This is owning cleanup, not source replay.
	if e := s.cleanupNormalization(); e != nil {
		return e
	}
	s.normalizationOwned = true
	e := m.setRationalWithBeginReceipt(out, ebits, sbits, mode, &s.q, &s.child, &s.childFrame, &s.childInvocation)
	if s.childFrame != nil && s.childFrame.invocation == s.childInvocation {
		s.exponentFrame = s.childFrame.childFrame
		s.exponentInvocation = s.childFrame.childInvocation
		s.childExpFinished = !s.childFrame.expLive
	}
	if e != nil {
		return e
	}
	if e := s.cleanupChild(); e != nil {
		return e
	}
	s.childOwned = true
	return nil
}

type arbitraryFloatSignedFractionScope struct {
	manager                                          *ArbitraryFloatManager
	invocation                                       uint64
	q                                                Rational
	qLive                                            bool
	qCursor                                          uint8
	child                                            ArbitraryFloatPlainRationalSetReceipt
	childFrame                                       *arbitraryFloatPlainRationalSetScope
	childInvocation                                  uint64
	exponentFrame                                    *arbitraryFloatRationalSetScope
	exponentInvocation                               uint64
	childExpFinished, childOwned, normalizationOwned bool
	hooks                                            NumericInvocationHooks
	numeric                                          NumericAllocationRoots
	gcd, division                                    integerRawInvocationReceipt

	integerBinding, naturalBinding NumericCallerBinding
	active, executing, finishing   bool
	bodyCause, sourceCause         error
}

func (s *arbitraryFloatSignedFractionScope) valid() bool {
	return s != nil && s.manager != nil && s.manager.self == s.manager && s == &s.manager.signedFraction && s.invocation != 0
}
func (s *arbitraryFloatSignedFractionScope) NumericExecuting() bool {
	return s.valid() && (s.executing || s.finishing)
}
func (s *arbitraryFloatSignedFractionScope) NumericBound(executing bool) bool {
	return s.valid() && (!executing || s.active && s.executing && !s.finishing && (s.manager.memory == nil || !s.manager.memory.FailureTransportActive()))
}
func (s *arbitraryFloatSignedFractionScope) NumericRoots(z *IntegerManager, local *NumericAllocationRoots) *NumericAllocationRoots {
	if !s.NumericBound(true) || z != s.manager.integer {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.numeric
}
func (s *arbitraryFloatSignedFractionScope) NumericRecord(z *IntegerManager, event string, r NumericInvocationReceipt) {
	if !s.NumericBound(true) || z != s.manager.integer || r.Raw == nil {
		return
	}
	switch event {
	case "gcd":
		s.gcd = *r.Raw
	case "division":
		s.division = *r.Raw
	}
}

func (s *arbitraryFloatSignedFractionScope) bind() error {
	z := s.manager.integer
	if e := z.BindCaller(&s.integerBinding, s, 37); e != nil {
		return e
	}
	if e := z.Natural.BindCaller(&s.naturalBinding, s, 37); e != nil {
		return JoinNumericErrors(e, s.integerBinding.Unbind())
	}
	return nil
}
func (s *arbitraryFloatSignedFractionScope) unbind() error {
	return JoinNumericErrors(s.naturalBinding.Unbind(), s.integerBinding.Unbind())
}
func (s *arbitraryFloatSignedFractionScope) cleanupNormalization() error {
	z := s.manager.integer

	if e := z.CleanupRawInvocation(&s.division, true); e != nil {
		return e
	}
	if e := z.CleanupRawInvocation(&s.gcd, true); e != nil {
		return e
	}

	return s.numeric.Cleanup(z)
}
func (s *arbitraryFloatSignedFractionScope) childSource() error {
	m := s.manager
	if s.child.frame != nil {
		_, source, _ := m.PlainRationalSetInvocationCauses(&s.child)
		return source
	}
	if s.childFrame != nil && s.childFrame.invocation == s.childInvocation {
		return s.childFrame.sourceCause
	}
	if s.exponentFrame != nil && s.exponentFrame.invocation == s.exponentInvocation {
		return s.exponentFrame.sourceCause
	}
	return nil
}
func (s *arbitraryFloatSignedFractionScope) cleanupChild() error {
	m := s.manager
	if s.child.frame != nil {
		return m.CleanupPlainRationalSetWithReceipt(&s.child, true)
	}
	if s.childFrame == nil {
		return nil
	}
	// Inline and pre-history failures still have a real exponent child. Join
	// only its captured physical frame; nil public receipts also sweep older
	// histories and cannot authorize this parent to retire unrelated callers.
	if s.exponentFrame == &m.rationalSet && s.exponentInvocation != 0 && s.exponentFrame.invocation == s.exponentInvocation {
		if e := m.cleanupExponentRationalSetScope(nil, true, true); e != nil {
			return e
		}
	}
	if s.childFrame == &m.plainRationalSet && s.childFrame.invocation == s.childInvocation {
		if s.childFrame.expLive {
			if e := m.integer.CleanupRawNumeral(&s.childFrame.exp); e != nil {
				return e
			}
			s.childFrame.expLive = false
		}
		s.childFrame.active = false
		s.childFrame.recordPlainHistory()
		return nil
	}
	if !s.childExpFinished {
		return ErrNumericOwner
	}
	return nil
}
func (m *ArbitraryFloatManager) finishSignedFractionSource() error {
	s := &m.signedFraction
	if s.executing || s.finishing {
		return ErrNumericPending
	}
	if !s.active {
		return nil
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	s.finishing = true
	defer func() { s.finishing = false }()
	if !s.childOwned {
		if e := s.childSource(); e != nil {
			s.sourceCause = e
			return e
		}
	}
	if !s.normalizationOwned {
		for _, e := range []error{m.integer.RawInvocationSourceCause(&s.division), m.integer.RawInvocationSourceCause(&s.gcd)} {
			if e != nil {
				s.sourceCause = e
				return e
			}
		}
	}
	for s.qLive && s.qCursor < 2 {
		v := &s.q.Num
		if s.qCursor == 1 {
			v = &s.q.Den
		}
		if _, e := SourceNumeralDel(m.integer, v); e != nil {
			s.sourceCause = e
			return e
		}
		s.qCursor++
	}
	s.qLive = false
	return nil
}

// CleanupSignedFraction joins only actually begun normalization/conversion
// children and scoped fields, preserving any stopped source destruction.
func (m *ArbitraryFloatManager) CleanupSignedFraction() (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.signedFraction
	if s.executing || s.finishing || m.valueExecuting || m.memory != nil && m.memory.FailureTransportActive() {
		return ErrNumericPending
	}
	if !s.active {
		return nil
	}
	if !s.valid() {
		return ErrNumericOwner
	}
	if e := s.bind(); e != nil {
		return e
	}
	s.finishing = true
	defer func() { s.finishing = false; cause = JoinNumericErrors(cause, s.unbind()) }()
	if !s.childOwned {
		if e := s.cleanupChild(); e != nil {
			return e
		}
		s.childOwned = true
	}
	if !s.normalizationOwned {
		if e := s.cleanupNormalization(); e != nil {
			return e
		}
		s.normalizationOwned = true
	}
	for s.qLive && s.qCursor < 2 {
		v := &s.q.Num
		if s.qCursor == 1 {
			v = &s.q.Den
		}
		if e := m.integer.CleanupRawNumeral(v); e != nil {
			return e
		}
		s.qCursor++
	}
	s.qLive = false
	s.active = false
	return nil
}
