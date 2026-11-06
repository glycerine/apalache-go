package numeral

// Source: src/util/mpf.cpp:1228-1253; src/util/mpf.h:218.

import (
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"math"
	// ToIEEEBVMPZ assembles the stored sign, biased exponent and significand.
	// The manager's Integer namespace owns out; input is not unpacked.
)

func (m *ArbitraryFloatManager) ToIEEEBVMPZ(x *ArbitraryFloat, out *Int) (cause error) {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	if m.ieeeBV.active {
		return ErrNumericPending
	}
	if e := m.valueGuard(x); e != nil {
		return e
	}
	if e := m.integer.check(out); e != nil {
		return e
	}
	assertionsEnabled, e := mpfSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if assertionsEnabled {
		nan, e := m.IsNaN(x)
		if e != nil {
			return e
		}
		if e := mpfSourceAssertion(!nan, 1229, "!is_nan(x)"); e != nil {
			return e
		}
	}
	if e := mpfSourceAssertion(x.exponent < math.MaxInt32, 1230, "exp(x) < INT_MAX"); e != nil {
		return e
	}
	s := &m.ieeeBV
	if s.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	*s = arbitraryFloatIEEEBVScope{manager: m, invocation: s.invocation + 1, active: true, executing: true}
	s.hooks.Policy = s
	if e := s.bind(); e != nil {
		s.executing = false
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = JoinNumericErrors(cause, m.finishIEEEBVSource())
		cause = JoinNumericErrors(cause, s.unbind())
	}()
	if e := s.run(x, out); e != nil {
		return e
	}
	return m.traceFloatDebugIntChecked("to_ieee_bv_mpz", "IEEE_BV = ", out, 1252)
}

func (s *arbitraryFloatIEEEBVScope) run(x *ArbitraryFloat, out *Int) error {
	if !s.NumericBound(true) {
		return ErrNumericOwner
	}
	m, z, c := s.manager, s.manager.integer, s.context()
	sb, eb := x.SBits(), x.EBits()
	inf, e := m.IsInf(x)
	if e != nil {
		return e
	}
	if inf {
		if e := s.setSign(x, out); e != nil {
			return e
		}
		if e := z.mul2kCoreWithAllocationContext(out, eb, c); e != nil {
			return e
		}
		w, e := m.powerTwoMinusOneWrapper(eb, false)
		if e != nil {
			return e
		}
		exp, e := w.Capture()
		if e != nil {
			return e
		}
		if e := z.addSubWithAllocationContext(out, &exp, out, false, c); e != nil {
			return e
		}
		return z.mul2kCoreWithAllocationContext(out, sb-1, c)
	}
	s.biasedExpLive = true
	bias, e := m.BiasExp(eb, x.exponent)
	if e != nil {
		return e
	}
	if e := z.setInt64WithAllocationContext(&s.biasedExp, bias, c); e != nil {
		return e
	}
	if e := s.setSign(x, out); e != nil {
		return e
	}
	if e := z.mul2kCoreWithAllocationContext(out, eb, c); e != nil {
		return e
	}
	if e := z.addSubWithAllocationContext(out, &s.biasedExp, out, false, c); e != nil {
		return e
	}
	if e := z.mul2kCoreWithAllocationContext(out, sb-1, c); e != nil {
		return e
	}
	return z.addSubWithAllocationContext(out, &x.significand, out, false, c)
}

func (s *arbitraryFloatIEEEBVScope) setSign(x *ArbitraryFloat, out *Int) error {
	v := int64(0)
	if x.Sign() {
		v = 1
	}
	return s.manager.integer.setInt64WithAllocationContext(out, v, s.context())
}
