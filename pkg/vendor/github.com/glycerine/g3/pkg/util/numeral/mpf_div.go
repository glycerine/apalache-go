package numeral

// Source: src/util/mpf.cpp:667-746; src/util/mpf.h:127.

func (m *ArbitraryFloatManager) Div(mode ArbitraryFloatRoundingMode, x, y, out *ArbitraryFloat) (cause error) {
	for _, v := range [...]*ArbitraryFloat{x, y, out} {
		if e := m.valueGuard(v); e != nil {
			return e
		}
	}
	if m.div.active {
		return ErrNumericPending
	}
	if e := mpfSourceAssertion(x.SBits() == y.SBits() && x.EBits() == y.EBits(), 668, "x.sbits == y.sbits && x.ebits == y.ebits"); e != nil {
		return e
	}
	if e := m.traceFloatDebugValueChecked("div", "X = ", x, 670); e != nil {
		return e
	}
	if e := m.traceFloatDebugValueChecked("div", "Y = ", y, 671); e != nil {
		return e
	}
	for _, v := range [...]*ArbitraryFloat{x, y} {
		nan, e := m.IsNaN(v)
		if e != nil {
			return e
		}
		if nan {
			return m.MkNaN(x.EBits(), x.SBits(), out, nil)
		}
	}
	// Preserve the four ordered source infinity queries and inner tests.
	for _, branch := range [...]struct {
		v, other            *ArbitraryFloat
		negative, numerator bool
	}{{x, y, false, true}, {y, x, false, false}, {x, y, true, true}, {y, x, true, false}} {
		var inf bool
		var e error
		if branch.negative {
			inf, e = m.IsNegativeInf(branch.v)
		} else {
			inf, e = m.IsPositiveInf(branch.v)
		}
		if e != nil {
			return e
		}
		if inf {
			otherInf, e := m.IsInf(branch.other)
			if e != nil {
				return e
			}
			if otherInf {
				return m.MkNaN(x.EBits(), x.SBits(), out, nil)
			}
			if branch.numerator {
				return m.MkInf(x.EBits(), x.SBits(), branch.other.Sign() != branch.negative, out)
			}
			return m.MkZero(x.EBits(), x.SBits(), x.Sign() != y.Sign(), out)
		}
	}
	zero, e := m.IsZero(y)
	if e != nil {
		return e
	}
	if zero {
		otherZero, e := m.IsZero(x)
		if e != nil {
			return e
		}
		if otherZero {
			return m.MkNaN(x.EBits(), x.SBits(), out, nil)
		}
		return m.MkInf(x.EBits(), x.SBits(), x.Sign() != y.Sign(), out)
	}
	zero, e = m.IsZero(x)
	if e != nil {
		return e
	}
	if zero {
		return m.MkZero(x.EBits(), x.SBits(), x.Sign() != y.Sign(), out)
	}
	out.assignEbits(x.EBits())
	out.assignSbits(x.SBits())
	out.assignSign(x.Sign() != y.Sign())
	s := &m.div
	if s.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	*s = arbitraryFloatDivScope{manager: m, invocation: s.invocation + 1, active: true, executing: true, aLive: true, bLive: true}
	s.hooks.Policy = s
	if e := s.bind(); e != nil {
		s.executing = false
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = JoinNumericErrors(cause, m.finishDivSource())
		cause = JoinNumericErrors(cause, s.unbind())
	}()
	context := s.context()
	z := m.integer
	if e := m.copyWithValueReceiptAndContext(&s.a, x, &s.copyA, context); e != nil {
		return e
	}
	if e := m.copyWithValueReceiptAndContext(&s.b, y, &s.copyB, context); e != nil {
		return e
	}
	if e := m.unpackRationalNumeral(&s.a, true, context); e != nil {
		return e
	}
	if e := m.unpackRationalNumeral(&s.b, true, context); e != nil {
		return e
	}
	if e := m.traceFloatDebugValueChecked("div", "A = ", &s.a, 722); e != nil {
		return e
	}
	if e := m.traceFloatDebugValueChecked("div", "B = ", &s.b, 723); e != nil {
		return e
	}
	exponent, e := roundSignedSub(s.a.exponent, s.b.exponent)
	if e != nil {
		return e
	}
	out.exponent = exponent
	if e := m.traceFloatDebugIntChecked("div", "A' = ", &s.a.significand, 727); e != nil {
		return e
	}
	if e := m.traceFloatDebugIntChecked("div", "B' = ", &s.b.significand, 728); e != nil {
		return e
	}
	extraBits := x.SBits() + 2
	// This source call uses the two-argument in-place overload, without SET.
	if e := z.check(&s.a.significand); e != nil {
		return e
	}
	if e := z.mul2kCoreWithAllocationContext(&s.a.significand, x.SBits()+extraBits, context); e != nil {
		return e
	}
	if e := z.machineDivWithAllocationContext(&s.a.significand, &s.b.significand, &out.significand, context); e != nil {
		return e
	}
	if e := m.traceFloatDebugValueChecked("div", "QUOTIENT = ", out, 734); e != nil {
		return e
	}
	s.stickyLive = true
	power, e := s.power(extraBits - 2)
	if e != nil {
		return e
	}
	if e := z.machineDivRemWithAllocationContext(&out.significand, &power, &out.significand, &s.sticky, context); e != nil {
		return e
	}
	empty, e := z.IsZero(&s.sticky)
	if e != nil {
		return e
	}
	if !empty {
		even, e := z.IsEven(&out.significand)
		if e != nil {
			return e
		}
		if even {
			one := NewInt(1)
			if e := z.addSubWithAllocationContext(&out.significand, &one, &out.significand, false, context); e != nil {
				return e
			}
		}
	}
	if e := m.traceFloatDebugValueChecked("div", "QUOTIENT' = ", out, 742); e != nil {
		return e
	}
	return m.roundWithParentContext(mode, out, &s.roundReceipt, context)
}
