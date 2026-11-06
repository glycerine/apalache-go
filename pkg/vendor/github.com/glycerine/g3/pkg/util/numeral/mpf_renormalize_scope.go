package numeral

// Source: src/util/mpf.cpp:1255-1270 (no lexical NUM or RAW declarations).

import

// Cached powers retain their existing persistent member owners. This scope
// owns only allocating prefixes reached by the caller's in-place shift.
umemory "github.com/glycerine/g3/pkg/util/memory"

type arbitraryFloatRenormalizeScope struct {
	manager                        *ArbitraryFloatManager
	invocation                     uint64
	hooks                          NumericInvocationHooks
	integerBinding, naturalBinding NumericCallerBinding
	numeric                        NumericAllocationRoots
	active, executing, finishing   bool
	bodyCause                      error
}

func (s *arbitraryFloatRenormalizeScope) valid() bool {
	return s != nil && s.manager != nil && s.manager.self == s.manager && s == &s.manager.renormalize && s.invocation != 0
}
func (s *arbitraryFloatRenormalizeScope) NumericExecuting() bool {
	return s.valid() && (s.executing || s.finishing)
}
func (s *arbitraryFloatRenormalizeScope) NumericBound(executing bool) bool {
	return s.valid() && (!executing || s.active && s.executing && !s.finishing && !s.manager.memory.FailureTransportActive())
}
func (s *arbitraryFloatRenormalizeScope) NumericRoots(z *IntegerManager, local *NumericAllocationRoots) *NumericAllocationRoots {
	if !s.NumericBound(true) || z != s.manager.integer {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.numeric
}
func (s *arbitraryFloatRenormalizeScope) context() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{Caller: &s.hooks}
}
func (s *arbitraryFloatRenormalizeScope) bind() error {
	z := s.manager.integer
	if e := z.BindCaller(&s.integerBinding, s, 21); e != nil {
		return e
	}
	if e := z.Natural.BindCaller(&s.naturalBinding, s, 21); e != nil {
		return JoinNumericErrors(e, s.integerBinding.Unbind())
	}
	return nil
}
func (s *arbitraryFloatRenormalizeScope) unbind() error {
	return JoinNumericErrors(s.naturalBinding.Unbind(), s.integerBinding.Unbind())
}
func (m *ArbitraryFloatManager) CleanupRenormalize() (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.renormalize
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
	if e := s.numeric.Cleanup(m.integer); e != nil {
		return e
	}
	s.active = false
	return nil
}
