package numeral

// Source: src/util/mpf.cpp:951-1003 (lower, upper, mid, product, diff lifetimes).

import umemory "github.com/glycerine/g3/pkg/util/memory"

type integerMPFSqrtHelperScope struct {
	manager                        *IntegerManager
	invocation                     uint64
	hooks                          NumericInvocationHooks
	integerBinding, naturalBinding NumericCallerBinding
	numeric                        NumericAllocationRoots

	copy                         integerCopyAllocationReceipt
	values                       [5]Int
	live                         [5]bool
	active, executing, finishing bool
	bodyCause, sourceCause       error
}

func (s *integerMPFSqrtHelperScope) valid() bool {
	return s != nil && s.manager != nil && s.manager.self == s.manager && s == &s.manager.mySqrt && s.invocation != 0
}
func (s *integerMPFSqrtHelperScope) NumericExecuting() bool {
	return s.valid() && (s.executing || s.finishing)
}
func (s *integerMPFSqrtHelperScope) NumericBound(executing bool) bool {
	return s.valid() && (!executing || s.active && s.executing && !s.finishing && !s.manager.Memory.FailureTransportActive())
}
func (s *integerMPFSqrtHelperScope) NumericRoots(z *IntegerManager, local *NumericAllocationRoots) *NumericAllocationRoots {
	if !s.NumericBound(true) || z != s.manager {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.numeric
}

func (s *integerMPFSqrtHelperScope) context() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{Caller: &s.hooks}
}
func (s *integerMPFSqrtHelperScope) bind() error {
	z := s.manager
	if e := z.BindCaller(&s.integerBinding, s, 20); e != nil {
		return e
	}
	if e := z.Natural.BindCaller(&s.naturalBinding, s, 20); e != nil {
		return JoinNumericErrors(e, s.integerBinding.Unbind())
	}
	return nil
}
func (s *integerMPFSqrtHelperScope) unbind() error {
	return JoinNumericErrors(s.naturalBinding.Unbind(), s.integerBinding.Unbind())
}
func (m *IntegerManager) finishMyMPZSqrtSource() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.mySqrt
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
			if _, e := SourceNumeralDel(m, local.value); e != nil {
				s.sourceCause = e
				return e
			}
			*local.live = false
		}
	}
	s.active = s.bodyCause != nil || NumericAllocationPending(&s.numeric)

	return nil
}

func (m *IntegerManager) CleanupMyMPZSqrt() (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.mySqrt
	if s.executing || s.finishing || m.Memory != nil && m.Memory.FailureTransportActive() {
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
	z := m

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
