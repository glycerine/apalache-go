package numeral

// Source: src/util/mpf.cpp:805-946 (mr, a, b, c, sticky_rem, copied res and Round lifetimes).

import umemory "github.com/glycerine/g3/pkg/util/memory"

type arbitraryFloatFMAScope struct {
	manager                        *ArbitraryFloatManager
	invocation                     uint64
	hooks                          NumericInvocationHooks
	integerBinding, naturalBinding NumericCallerBinding
	numeric                        NumericAllocationRoots

	copyA, copyB, copyC, copyRes                                    ArbitraryFloatValueCopyReceipt
	roundFrame                                                      *ArbitraryFloatRoundScope
	roundInvocation                                                 uint64
	roundReceipt                                                    ArbitraryFloatRoundReceipt
	mr, a, b, c, res                                                ArbitraryFloat
	sticky                                                          Int
	mrLive, aLive, bLive, cLive, stickyLive, resLive, resSourceLive bool
	active, executing, finishing                                    bool
	bodyCause, sourceCause                                          error
}

func (s *arbitraryFloatFMAScope) valid() bool {
	return s != nil && s.manager != nil && s.manager.self == s.manager && s == &s.manager.fma && s.invocation != 0
}
func (s *arbitraryFloatFMAScope) NumericExecuting() bool {
	return s.valid() && (s.executing || s.finishing)
}
func (s *arbitraryFloatFMAScope) NumericBound(executing bool) bool {
	return s.valid() && (!executing || s.active && s.executing && !s.finishing && !s.manager.memory.FailureTransportActive())
}
func (s *arbitraryFloatFMAScope) NumericRoots(z *IntegerManager, local *NumericAllocationRoots) *NumericAllocationRoots {
	if !s.NumericBound(true) || z != s.manager.integer {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.numeric
}

func (s *arbitraryFloatFMAScope) context() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{Caller: &s.hooks}
}
func (s *arbitraryFloatFMAScope) power(n uint32) (Int, error) {
	w, e := s.manager.powerTwoWrapper(n, false)
	if e != nil {
		return Int{}, e
	}
	return w.Capture()
}
func (s *arbitraryFloatFMAScope) bind() error {
	z := s.manager.integer
	if e := z.BindCaller(&s.integerBinding, s, 18); e != nil {
		return e
	}
	if e := z.Natural.BindCaller(&s.naturalBinding, s, 18); e != nil {
		return JoinNumericErrors(e, s.integerBinding.Unbind())
	}
	return nil
}
func (s *arbitraryFloatFMAScope) unbind() error {
	return JoinNumericErrors(s.naturalBinding.Unbind(), s.integerBinding.Unbind())
}
func (m *ArbitraryFloatManager) finishFMASource() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.fma
	if s.executing || s.finishing {
		return ErrNumericPending
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if !s.active {
		return nil
	}
	s.finishing = true
	defer func() { s.finishing = false }()
	if s.roundFrame != nil {
		if s.roundFrame != &m.RoundScope || s.roundInvocation != m.RoundScope.Invocation {
			return umemory.ErrRuntimeOwner
		}
		if s.roundFrame.sourceCause != nil {
			s.sourceCause = s.roundFrame.sourceCause
			return s.sourceCause
		}
	}

	for _, local := range [...]struct {
		value *Int
		live  *bool
	}{{&s.res.significand, &s.resSourceLive}, {&s.sticky, &s.stickyLive}, {&s.c.significand, &s.cLive}, {&s.b.significand, &s.bLive}, {&s.a.significand, &s.aLive}, {&s.mr.significand, &s.mrLive}} {
		if *local.live {
			if _, e := SourceNumeralDel(m.integer, local.value); e != nil {
				s.sourceCause = e
				return e
			}
			*local.live = false
			if local.value == &s.res.significand {
				s.resLive = false
			}
		}
	}
	s.active = s.bodyCause != nil || NumericAllocationPending(&s.numeric) || s.roundFrame != nil && s.roundFrame.active

	return nil
}

func (m *ArbitraryFloatManager) CleanupFMA() (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.fma
	if s.executing || s.finishing || m.valueExecuting || m.memory != nil && m.memory.FailureTransportActive() {
		return ErrNumericPending
	}
	if !s.active {
		return nil
	}
	if !s.valid() {
		return umemory.ErrRuntimeOwner
	}
	if e := s.bind(); e != nil {
		return e
	}
	s.finishing = true
	defer func() { s.finishing = false; cause = JoinNumericErrors(cause, s.unbind()) }()
	z := m.integer
	if s.roundFrame != nil {
		if s.roundFrame != &m.RoundScope || s.roundInvocation != m.RoundScope.Invocation {
			return umemory.ErrRuntimeOwner
		}
		if e := m.CleanupRound(); e != nil {
			return e
		}
	}

	if e := m.CleanupValueCopy(&s.copyRes, false); e != nil {
		return e
	}
	if e := m.CleanupValueCopy(&s.copyC, false); e != nil {
		return e
	}
	if e := m.CleanupValueCopy(&s.copyB, false); e != nil {
		return e
	}
	if e := m.CleanupValueCopy(&s.copyA, false); e != nil {
		return e
	}
	for _, local := range [...]struct {
		value *Int
		live  *bool
	}{{&s.res.significand, &s.resLive}, {&s.sticky, &s.stickyLive}, {&s.c.significand, &s.cLive}, {&s.b.significand, &s.bLive}, {&s.a.significand, &s.aLive}, {&s.mr.significand, &s.mrLive}} {
		if *local.live {
			if e := z.CleanupRawNumeral(local.value); e != nil {
				return e
			}
			*local.live = false
		}
	}
	if e := m.CleanupValueCopy(&s.copyRes, true); e != nil {
		return e
	}
	if e := m.CleanupValueCopy(&s.copyC, true); e != nil {
		return e
	}
	if e := m.CleanupValueCopy(&s.copyB, true); e != nil {
		return e
	}
	if e := m.CleanupValueCopy(&s.copyA, true); e != nil {
		return e
	}

	if e := s.numeric.Cleanup(z); e != nil {
		return e
	}
	s.active = false
	return nil
}

// The actual Round entry stamps its new child tuple, including nonallocating
// invocations for which no history receipt is materialized.
func (s *arbitraryFloatFMAScope) enterArbitraryFloatRound(m *ArbitraryFloatManager, frame *ArbitraryFloatRoundScope) error {
	if !s.NumericBound(true) || m != s.manager || frame != &m.RoundScope || !frame.active || !frame.executing {
		return umemory.ErrRuntimeOwner
	}
	s.roundFrame = frame
	s.roundInvocation = frame.Invocation
	return nil
}
