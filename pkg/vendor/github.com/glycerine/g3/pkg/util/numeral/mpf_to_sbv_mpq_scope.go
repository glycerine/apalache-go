package numeral

// Source: src/util/mpf.cpp:1186-1223 (t and z scoped lifetimes).

import umemory "github.com/glycerine/g3/pkg/util/memory"

type arbitraryFloatSBVScope struct {
	manager                        *ArbitraryFloatManager
	invocation                     uint64
	hooks                          NumericInvocationHooks
	integerBinding, naturalBinding NumericCallerBinding
	numeric                        NumericAllocationRoots

	copyA                        ArbitraryFloatValueCopyReceipt
	a                            ArbitraryFloat
	z                            Int
	copyZ, outputCopy            integerCopyAllocationReceipt
	aLive, zLive                 bool
	active, executing, finishing bool
	bodyCause, sourceCause       error
}

func (s *arbitraryFloatSBVScope) valid() bool {
	return s != nil && s.manager != nil && s.manager.self == s.manager && s == &s.manager.sbv && s.invocation != 0
}
func (s *arbitraryFloatSBVScope) NumericExecuting() bool {
	return s.valid() && (s.executing || s.finishing)
}
func (s *arbitraryFloatSBVScope) NumericBound(executing bool) bool {
	return s.valid() && (!executing || s.active && s.executing && !s.finishing && !s.manager.memory.FailureTransportActive())
}
func (s *arbitraryFloatSBVScope) NumericRoots(z *IntegerManager, local *NumericAllocationRoots) *NumericAllocationRoots {
	if !s.NumericBound(true) || z != s.manager.integer {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.numeric
}

func (s *arbitraryFloatSBVScope) context() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{Caller: &s.hooks}
}
func (s *arbitraryFloatSBVScope) bind() error {
	z := s.manager.integer
	if e := z.BindCaller(&s.integerBinding, s, 25); e != nil {
		return e
	}
	if e := z.Natural.BindCaller(&s.naturalBinding, s, 25); e != nil {
		return JoinNumericErrors(e, s.integerBinding.Unbind())
	}
	return nil
}
func (s *arbitraryFloatSBVScope) unbind() error {
	return JoinNumericErrors(s.naturalBinding.Unbind(), s.integerBinding.Unbind())
}
func (m *ArbitraryFloatManager) finishSBVSource() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.sbv
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
	}{{&s.z, &s.zLive}, {&s.a.significand, &s.aLive}} {
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

func (m *ArbitraryFloatManager) CleanupToSBVMPQ() (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.sbv
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

	if e := m.CleanupValueCopy(&s.copyA, false); e != nil {
		return e
	}
	for _, local := range [...]struct {
		value *Int
		live  *bool
	}{{&s.z, &s.zLive}, {&s.a.significand, &s.aLive}} {
		if *local.live {
			if e := z.CleanupRawNumeral(local.value); e != nil {
				return e
			}
			*local.live = false
		}
	}
	if e := z.CleanupCopyAllocationInvocation(&s.outputCopy); e != nil {
		return e
	}
	if e := z.CleanupCopyAllocationInvocation(&s.copyZ); e != nil {
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
