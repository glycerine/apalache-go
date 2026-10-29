package numeral

// Source: src/util/mpf.cpp:1100-1170 (scoped_mpf and scoped_mpz lifetimes).

import umemory "github.com/glycerine/g3/pkg/util/memory"

type arbitraryFloatIntegralRoundScope struct {
	manager                        *ArbitraryFloatManager
	invocation                     uint64
	hooks                          NumericInvocationHooks
	integerBinding, naturalBinding NumericCallerBinding
	numeric                        NumericAllocationRoots

	copyA                        ArbitraryFloatValueCopyReceipt
	a                            ArbitraryFloat
	div, rem                     Int
	aLive, divLive, remLive      bool
	active, executing, finishing bool
	bodyCause, sourceCause       error
}

// This protects exactly the borrowed primitive of the executing MPF caller.
// Other numeric caller policies may intentionally allow an unrelated receiver
// to operate during returning transport; their existing policy still applies.
func (z *IntegerManager) mpfArithmeticTransportActive() bool {
	if z == nil || z.self != z {
		return false
	}
	m := z.MpfParent
	return m != nil && m.self == m && m.integer == z && (m.signedFraction.valid() && m.signedFraction.NumericExecuting() || m.hexfloatComponents.valid() && m.hexfloatComponents.NumericExecuting() || m.binaryString.valid() && m.binaryString.NumericExecuting() || m.decimalStream.valid() && m.decimalStream.NumericExecuting() || m.smt2Stream.valid() && m.smt2Stream.NumericExecuting() || m.rationalString.valid() && m.rationalString.NumericExecuting() || m.ieeeBV.valid() && m.ieeeBV.NumericExecuting() || m.sbv.valid() && m.sbv.NumericExecuting() || m.rem.valid() && m.rem.NumericExecuting() || m.partialRemainder.valid() && m.partialRemainder.NumericExecuting() || m.renormalize.valid() && m.renormalize.NumericExecuting() || m.integralRound.valid() && m.integralRound.NumericExecuting() || m.addSub.valid() && m.addSub.NumericExecuting() || m.mul.valid() && m.mul.NumericExecuting() || m.div.valid() && m.div.NumericExecuting() || m.fma.valid() && m.fma.NumericExecuting() || m.sqrt.valid() && m.sqrt.NumericExecuting()) && m.memory != nil && m.memory.FailureTransportActive()
}

func (s *arbitraryFloatIntegralRoundScope) valid() bool {
	return s != nil && s.manager != nil && s.manager.self == s.manager && s == &s.manager.integralRound && s.invocation != 0
}
func (s *arbitraryFloatIntegralRoundScope) NumericExecuting() bool {
	return s.valid() && (s.executing || s.finishing)
}
func (s *arbitraryFloatIntegralRoundScope) NumericBound(executing bool) bool {
	return s.valid() && (!executing || s.active && s.executing && !s.finishing && !s.manager.memory.FailureTransportActive())
}
func (s *arbitraryFloatIntegralRoundScope) NumericRoots(z *IntegerManager, local *NumericAllocationRoots) *NumericAllocationRoots {
	if !s.NumericBound(true) || z != s.manager.integer {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.numeric
}

func (s *arbitraryFloatIntegralRoundScope) context() integerAllocationInvocationContext {
	return integerAllocationInvocationContext{Caller: &s.hooks}
}
func (s *arbitraryFloatIntegralRoundScope) power(n uint32) (Int, error) {
	w, e := s.manager.powerTwoWrapper(n, false)
	if e != nil {
		return Int{}, e
	}
	return w.Capture()
}
func (s *arbitraryFloatIntegralRoundScope) bind() error {
	z := s.manager.integer
	if e := z.BindCaller(&s.integerBinding, s, 14); e != nil {
		return e
	}
	if e := z.Natural.BindCaller(&s.naturalBinding, s, 14); e != nil {
		return JoinNumericErrors(e, s.integerBinding.Unbind())
	}
	return nil
}
func (s *arbitraryFloatIntegralRoundScope) unbind() error {
	return JoinNumericErrors(s.naturalBinding.Unbind(), s.integerBinding.Unbind())
}
func (m *ArbitraryFloatManager) finishIntegralRoundSource() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.integralRound
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
	}{{&s.rem, &s.remLive}, {&s.div, &s.divLive}, {&s.a.significand, &s.aLive}} {
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

func (m *ArbitraryFloatManager) CleanupRoundToIntegral() (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.integralRound
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
	}{{&s.rem, &s.remLive}, {&s.div, &s.divLive}, {&s.a.significand, &s.aLive}} {
		if *local.live {
			if e := z.CleanupRawNumeral(local.value); e != nil {
				return e
			}
			*local.live = false
		}
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
