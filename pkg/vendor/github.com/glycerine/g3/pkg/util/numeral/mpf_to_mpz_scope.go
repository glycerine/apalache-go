package numeral

// Source: src/util/mpf.cpp:1170-1181 (no lexical NUM or RAW declarations).

import

// The source numeral and output remain borrowed. This actual reflected scope
// owns only reached output COPY/shift prefixes, never a mirrored public value.
umemory "github.com/glycerine/g3/pkg/util/memory"

type arbitraryFloatToMPZScope struct {
	manager                                    *ArbitraryFloatManager
	output                                     *IntegerManager
	invocation                                 uint64
	hooks                                      NumericInvocationHooks
	sourceBinding, outputBinding               NumericCallerBinding
	sourceNaturalBinding, outputNaturalBinding NumericCallerBinding
	numeric                                    NumericAllocationRoots
	copy                                       integerCopyAllocationReceipt
	active, executing, finishing               bool
	bodyCause                                  error
}

func (s *arbitraryFloatToMPZScope) valid() bool {
	return s != nil && s.manager != nil && s.manager.self == s.manager && s == &s.manager.toMPZ && s.invocation != 0 && s.output != nil && s.output.self == s.output
}
func (s *arbitraryFloatToMPZScope) transportActive() bool {
	return s.valid() && (s.manager.memory != nil && s.manager.memory.FailureTransportActive() || s.output.Memory != nil && s.output.Memory.FailureTransportActive())
}
func (s *arbitraryFloatToMPZScope) NumericExecuting() bool {
	return s.valid() && (s.executing || s.finishing)
}
func (s *arbitraryFloatToMPZScope) NumericBound(executing bool) bool {
	return s.valid() && (!executing || s.active && s.executing && !s.finishing && !s.transportActive())
}
func (s *arbitraryFloatToMPZScope) NumericRoots(z *IntegerManager, local *NumericAllocationRoots) *NumericAllocationRoots {
	if !s.NumericBound(true) || z != s.output {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.numeric
}
func (s *arbitraryFloatToMPZScope) context() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{Caller: &s.hooks}
}
func (s *arbitraryFloatToMPZScope) bind() error {
	z := s.manager.integer
	if e := z.BindCaller(&s.sourceBinding, s, 24); e != nil {
		return e
	}
	if e := z.Natural.BindCaller(&s.sourceNaturalBinding, s, 24); e != nil {
		return JoinNumericErrors(e, s.sourceBinding.Unbind())
	}
	if e := s.output.BindCaller(&s.outputBinding, s, 24); e != nil {
		return JoinNumericErrors(JoinNumericErrors(e, s.sourceNaturalBinding.Unbind()), s.sourceBinding.Unbind())
	}
	if e := s.output.Natural.BindCaller(&s.outputNaturalBinding, s, 24); e != nil {
		return JoinNumericErrors(JoinNumericErrors(JoinNumericErrors(e, s.outputBinding.Unbind()), s.sourceNaturalBinding.Unbind()), s.sourceBinding.Unbind())
	}
	return nil
}
func (s *arbitraryFloatToMPZScope) unbind() error {
	cause := JoinNumericErrors(s.outputNaturalBinding.Unbind(), s.outputBinding.Unbind())
	cause = JoinNumericErrors(cause, s.sourceNaturalBinding.Unbind())
	return JoinNumericErrors(cause, s.sourceBinding.Unbind())
}
func (z *IntegerManager) mpfToMPZExecuting(transport bool) bool {
	if z == nil || z.self != z {
		return false
	}
	for b := z.callerBindings; b != nil; b = b.previous {
		s, ok := b.guard.(*arbitraryFloatToMPZScope)
		if b.self == b && ok && s.NumericExecuting() && (z == s.output || z == s.manager.integer) && (!transport || s.transportActive()) {
			return true
		}
	}
	return false
}
func (m *ArbitraryFloatManager) CleanupToMPZ() (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.toMPZ
	if s.executing || s.finishing || m.valueExecuting || s.transportActive() {
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
	if e := s.output.CleanupCopyAllocationInvocation(&s.copy); e != nil {
		return e
	}
	if e := s.numeric.Cleanup(s.output); e != nil {
		return e
	}
	s.active = false
	return nil
}
