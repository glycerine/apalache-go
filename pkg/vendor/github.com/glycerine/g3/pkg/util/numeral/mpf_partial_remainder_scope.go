package numeral

// Source: src/util/mpf.cpp:1272-1436 (actual reached scoped integer lifetimes).

import umemory "github.com/glycerine/g3/pkg/util/memory"

type arbitraryFloatPartialRemainderScope struct {
	manager                        *ArbitraryFloatManager
	invocation                     uint64
	hooks                          NumericInvocationHooks
	integerBinding, naturalBinding NumericCallerBinding
	numeric                        NumericAllocationRoots

	copy                         integerCopyAllocationReceipt
	values                       [13]Int
	live                         [13]bool
	active, executing, finishing bool
	bodyCause, sourceCause       error
}

func (s *arbitraryFloatPartialRemainderScope) valid() bool {
	return s != nil && s.manager != nil && s.manager.self == s.manager && s == &s.manager.partialRemainder && s.invocation != 0
}
func (s *arbitraryFloatPartialRemainderScope) NumericExecuting() bool {
	return s.valid() && (s.executing || s.finishing)
}
func (s *arbitraryFloatPartialRemainderScope) NumericBound(executing bool) bool {
	return s.valid() && (!executing || s.active && s.executing && !s.finishing && !s.manager.memory.FailureTransportActive())
}
func (s *arbitraryFloatPartialRemainderScope) NumericRoots(z *IntegerManager, local *NumericAllocationRoots) *NumericAllocationRoots {
	if !s.NumericBound(true) || z != s.manager.integer {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.numeric
}

func (s *arbitraryFloatPartialRemainderScope) context() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{Caller: &s.hooks}
}
func (s *arbitraryFloatPartialRemainderScope) bind() error {
	z := s.manager.integer
	if e := z.BindCaller(&s.integerBinding, s, 22); e != nil {
		return e
	}
	if e := z.Natural.BindCaller(&s.naturalBinding, s, 22); e != nil {
		return JoinNumericErrors(e, s.integerBinding.Unbind())
	}
	return nil
}
func (s *arbitraryFloatPartialRemainderScope) unbind() error {
	return JoinNumericErrors(s.naturalBinding.Unbind(), s.integerBinding.Unbind())
}
func (m *ArbitraryFloatManager) finishPartialRemainderSource() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.partialRemainder
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

	for i := len(s.values) - 1; i >= 0; i-- {
		local := struct {
			value *Int
			live  *bool
		}{&s.values[i], &s.live[i]}
		if *local.live {
			if _, e := SourceNumeralDel(m.integer, local.value); e != nil {
				s.sourceCause = e
				return e
			}
			*local.live = false
		}
	}
	s.active = s.bodyCause != nil || NumericAllocationPending(&s.numeric)

	return nil
}

func (m *ArbitraryFloatManager) CleanupPartialRemainder() (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.partialRemainder
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

	for i := len(s.values) - 1; i >= 0; i-- {
		local := struct {
			value *Int
			live  *bool
		}{&s.values[i], &s.live[i]}
		if *local.live {
			if e := z.CleanupRawNumeral(local.value); e != nil {
				return e
			}
			*local.live = false
		}
	}
	if e := z.CleanupCopyAllocationInvocation(&s.copy); e != nil {
		return e
	}

	if e := s.numeric.Cleanup(z); e != nil {
		return e
	}
	s.active = false
	return nil
}
