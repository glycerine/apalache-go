package numeral

// Source: src/util/mpf.cpp:1228-1253 (finite-branch biased_exp scoped lifetime).

import umemory "github.com/glycerine/g3/pkg/util/memory"

type arbitraryFloatIEEEBVScope struct {
	manager                        *ArbitraryFloatManager
	invocation                     uint64
	hooks                          NumericInvocationHooks
	integerBinding, naturalBinding NumericCallerBinding
	numeric                        NumericAllocationRoots

	biasedExp                    Int
	biasedExpLive                bool
	active, executing, finishing bool
	bodyCause, sourceCause       error
}

func (s *arbitraryFloatIEEEBVScope) valid() bool {
	return s != nil && s.manager != nil && s.manager.self == s.manager && s == &s.manager.ieeeBV && s.invocation != 0
}
func (s *arbitraryFloatIEEEBVScope) NumericExecuting() bool {
	return s.valid() && (s.executing || s.finishing)
}
func (s *arbitraryFloatIEEEBVScope) NumericBound(executing bool) bool {
	return s.valid() && (!executing || s.active && s.executing && !s.finishing && !s.manager.memory.FailureTransportActive())
}
func (s *arbitraryFloatIEEEBVScope) NumericRoots(z *IntegerManager, local *NumericAllocationRoots) *NumericAllocationRoots {
	if !s.NumericBound(true) || z != s.manager.integer {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.numeric
}

func (s *arbitraryFloatIEEEBVScope) context() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{Caller: &s.hooks}
}
func (s *arbitraryFloatIEEEBVScope) bind() error {
	z := s.manager.integer
	if e := z.BindCaller(&s.integerBinding, s, 26); e != nil {
		return e
	}
	if e := z.Natural.BindCaller(&s.naturalBinding, s, 26); e != nil {
		return JoinNumericErrors(e, s.integerBinding.Unbind())
	}
	return nil
}
func (s *arbitraryFloatIEEEBVScope) unbind() error {
	return JoinNumericErrors(s.naturalBinding.Unbind(), s.integerBinding.Unbind())
}
func (m *ArbitraryFloatManager) finishIEEEBVSource() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.ieeeBV
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

	for _, local := range [...]struct {
		value *Int
		live  *bool
	}{{&s.biasedExp, &s.biasedExpLive}} {
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

func (m *ArbitraryFloatManager) CleanupToIEEEBVMPZ() (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.ieeeBV
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

	for _, local := range [...]struct {
		value *Int
		live  *bool
	}{{&s.biasedExp, &s.biasedExpLive}} {
		if *local.live {
			if e := z.CleanupRawNumeral(local.value); e != nil {
				return e
			}
			*local.live = false
		}
	}

	if e := s.numeric.Cleanup(z); e != nil {
		return e
	}
	s.active = false
	return nil
}
