package numeral

// Source: src/util/mpf.cpp:716-744 (a, b, sticky_rem and Round lifetimes).

import umemory "github.com/glycerine/g3/pkg/util/memory"

type arbitraryFloatDivScope struct {
	manager                        *ArbitraryFloatManager
	invocation                     uint64
	hooks                          NumericInvocationHooks
	integerBinding, naturalBinding NumericCallerBinding
	numeric                        NumericAllocationRoots

	copyA, copyB                 ArbitraryFloatValueCopyReceipt
	roundFrame                   *ArbitraryFloatRoundScope
	roundInvocation              uint64
	roundReceipt                 ArbitraryFloatRoundReceipt
	a, b                         ArbitraryFloat
	sticky                       Int
	aLive, bLive, stickyLive     bool
	active, executing, finishing bool
	bodyCause, sourceCause       error
}

func (s *arbitraryFloatDivScope) valid() bool {
	return s != nil && s.manager != nil && s.manager.self == s.manager && s == &s.manager.div && s.invocation != 0
}
func (s *arbitraryFloatDivScope) NumericExecuting() bool {
	return s.valid() && (s.executing || s.finishing)
}
func (s *arbitraryFloatDivScope) NumericBound(executing bool) bool {
	return s.valid() && (!executing || s.active && s.executing && !s.finishing && !s.manager.memory.FailureTransportActive())
}
func (s *arbitraryFloatDivScope) NumericRoots(z *IntegerManager, local *NumericAllocationRoots) *NumericAllocationRoots {
	if !s.NumericBound(true) || z != s.manager.integer {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.numeric
}

func (s *arbitraryFloatDivScope) context() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{Caller: &s.hooks}
}
func (s *arbitraryFloatDivScope) power(n uint32) (Int, error) {
	w, e := s.manager.powerTwoWrapper(n, false)
	if e != nil {
		return Int{}, e
	}
	return w.Capture()
}
func (s *arbitraryFloatDivScope) bind() error {
	z := s.manager.integer
	if e := z.BindCaller(&s.integerBinding, s, 17); e != nil {
		return e
	}
	if e := z.Natural.BindCaller(&s.naturalBinding, s, 17); e != nil {
		return JoinNumericErrors(e, s.integerBinding.Unbind())
	}
	return nil
}
func (s *arbitraryFloatDivScope) unbind() error {
	return JoinNumericErrors(s.naturalBinding.Unbind(), s.integerBinding.Unbind())
}
func (m *ArbitraryFloatManager) finishDivSource() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.div
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
	}{{&s.sticky, &s.stickyLive}, {&s.b.significand, &s.bLive}, {&s.a.significand, &s.aLive}} {
		if *local.live {
			if _, e := SourceNumeralDel(m.integer, local.value); e != nil {
				s.sourceCause = e
				return e
			}
			*local.live = false
		}
	}
	s.active = s.bodyCause != nil || NumericAllocationPending(&s.numeric) || s.roundFrame != nil && s.roundFrame.active

	return nil
}

func (m *ArbitraryFloatManager) CleanupDiv() (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.div
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

	if e := m.CleanupValueCopy(&s.copyB, false); e != nil {
		return e
	}
	if e := m.CleanupValueCopy(&s.copyA, false); e != nil {
		return e
	}
	for _, local := range [...]struct {
		value *Int
		live  *bool
	}{{&s.sticky, &s.stickyLive}, {&s.b.significand, &s.bLive}, {&s.a.significand, &s.aLive}} {
		if *local.live {
			if e := z.CleanupRawNumeral(local.value); e != nil {
				return e
			}
			*local.live = false
		}
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
func (s *arbitraryFloatDivScope) enterArbitraryFloatRound(m *ArbitraryFloatManager, frame *ArbitraryFloatRoundScope) error {
	if !s.NumericBound(true) || m != s.manager || frame != &m.RoundScope || !frame.active || !frame.executing {
		return umemory.ErrRuntimeOwner
	}
	s.roundFrame = frame
	s.roundInvocation = frame.Invocation
	return nil
}
