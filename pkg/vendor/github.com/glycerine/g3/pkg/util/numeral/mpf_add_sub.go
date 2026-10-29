package numeral

// Source: src/util/mpf.cpp:470-588; src/util/mpf.h:124-125.

func (m *ArbitraryFloatManager) Add(mode ArbitraryFloatRoundingMode, x, y, out *ArbitraryFloat) error {
	return m.addSubtract(mode, x, y, out, false)
}
func (m *ArbitraryFloatManager) Sub(mode ArbitraryFloatRoundingMode, x, y, out *ArbitraryFloat) error {
	return m.addSubtract(mode, x, y, out, true)
}

func (m *ArbitraryFloatManager) addSubtract(mode ArbitraryFloatRoundingMode, x, y, out *ArbitraryFloat, sub bool) (cause error) {
	for _, v := range [...]*ArbitraryFloat{x, y, out} {
		if e := m.valueGuard(v); e != nil {
			return e
		}
	}
	if m.addSub.active {
		return ErrNumericPending
	}
	if e := mpfSourceAssertion(x.SBits() == y.SBits() && x.EBits() == y.EBits(), 479, "x.sbits == y.sbits && x.ebits == y.ebits"); e != nil {
		return e
	}
	signY := y.Sign() != sub
	nan, e := m.IsNaN(x)
	if e != nil {
		return e
	}
	if nan {
		return m.MkNaN(x.EBits(), x.SBits(), out, nil)
	}
	nan, e = m.IsNaN(y)
	if e != nil {
		return e
	}
	if nan {
		return m.MkNaN(x.EBits(), x.SBits(), out, nil)
	}
	inf, e := m.IsInf(x)
	if e != nil {
		return e
	}
	if inf {
		other, e := m.IsInf(y)
		if e != nil {
			return e
		}
		if other && x.Sign() != signY {
			return m.MkNaN(x.EBits(), x.SBits(), out, nil)
		}
		return m.Copy(out, x)
	}
	inf, e = m.IsInf(y)
	if e != nil {
		return e
	}
	if inf {
		other, e := m.IsInf(x)
		if e != nil {
			return e
		}
		if other && x.Sign() != signY {
			return m.MkNaN(x.EBits(), x.SBits(), out, nil)
		}
		if e := m.Copy(out, y); e != nil {
			return e
		}
		out.assignSign(signY)
		return nil
	}
	zeroX, e := m.IsZero(x)
	if e != nil {
		return e
	}
	if zeroX {
		zeroY, e := m.IsZero(y)
		if e != nil {
			return e
		}
		if zeroY {
			negative := x.Sign() && signY || mode == ArbitraryFloatTowardNegative && x.Sign() != signY
			return m.MkZero(x.EBits(), x.SBits(), negative, out)
		}
	}
	// These are separate source else-if queries, not cached classifications.
	zeroX, e = m.IsZero(x)
	if e != nil {
		return e
	}
	if zeroX {
		if e := m.Copy(out, y); e != nil {
			return e
		}
		out.assignSign(signY)
		return nil
	}
	zeroY, e := m.IsZero(y)
	if e != nil {
		return e
	}
	if zeroY {
		return m.Copy(out, x)
	}
	out.assignEbits(x.EBits())
	out.assignSbits(x.SBits())
	for _, assertion := range [...]struct {
		v          *ArbitraryFloat
		line       int32
		expression string
	}{{x, 518, "is_normal(x) || is_denormal(x)"}, {y, 519, "is_normal(y) || is_denormal(y)"}} {
		enabled, e := mpfSourceAssertionsEnabled()
		if e != nil {
			return e
		}
		if enabled {
			v := assertion.v
			normal, e := m.IsNormal(v)
			if e != nil {
				return e
			}
			denormal := false
			if !normal {
				denormal, e = m.IsDenormal(v)
				if e != nil {
					return e
				}
			}
			if e := mpfSourceAssertion(normal || denormal, assertion.line, assertion.expression); e != nil {
				return e
			}
		}
	}
	s := &m.addSub
	if s.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	*s = arbitraryFloatAddSubScope{manager: m, invocation: s.invocation + 1, active: true, executing: true, aLive: true, bLive: true}
	s.hooks.Policy = s
	if e := s.bind(); e != nil {
		s.executing = false
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = JoinNumericErrors(cause, m.finishAddSubSource())
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
	s.b.assignSign(signY)
	if e := m.unpackRationalNumeral(&s.a, false, context); e != nil {
		return e
	}
	if e := m.unpackRationalNumeral(&s.b, false, context); e != nil {
		return e
	}
	if s.b.exponent > s.a.exponent {
		s.a, s.b = s.b, s.a
	}
	delta, e := roundSignedSub(s.a.exponent, s.b.exponent)
	if e != nil {
		return e
	}
	if e := mpfSourceAssertion(s.a.exponent >= s.b.exponent, 535, "exp(a) >= exp(b)"); e != nil {
		return e
	}
	if e := mpfSourceAssertion(delta >= 0, 536, "exp_delta >= 0"); e != nil {
		return e
	}
	if delta > int64(x.SBits()+2) {
		delta = int64(x.SBits() + 2)
	}
	if e := m.traceFloatDebugValueChecked("add_sub", "A  = ", &s.a, 541); e != nil {
		return e
	}
	if e := m.traceFloatDebugValueChecked("add_sub", "B  = ", &s.b, 542); e != nil {
		return e
	}
	m.traceFloatDebugSigned("add_sub", "d  = ", delta, 543)
	if e := z.shiftWithAllocationContext(&s.a.significand, 3, &s.a.significand, true, context); e != nil {
		return e
	}
	if e := z.shiftWithAllocationContext(&s.b.significand, 3, &s.b.significand, true, context); e != nil {
		return e
	}
	if e := mpfSourceAssertion(delta <= 1<<31-1, 550, "exp_delta <= INT_MAX"); e != nil {
		return e
	}
	s.stickyLive = true
	p, e := s.power(uint32(delta))
	if e != nil {
		return e
	}
	if e := z.machineDivRemWithAllocationContext(&s.b.significand, &p, &s.b.significand, &s.sticky, context); e != nil {
		return e
	}
	if e := m.traceFloatDebugIntChecked("add_sub", "A' = ", &s.a.significand, 554); e != nil {
		return e
	}
	if e := m.traceFloatDebugIntChecked("add_sub", "B' = ", &s.b.significand, 555); e != nil {
		return e
	}
	subtract := s.a.Sign() != s.b.Sign()
	if subtract {
		m.traceFloatDebugText("add_sub", "SUBTRACTING", 559)
	} else {
		m.traceFloatDebugText("add_sub", "ADDING", 565)
	}
	if e := z.addSubWithAllocationContext(&s.a.significand, &s.b.significand, &out.significand, subtract, context); e != nil {
		return e
	}
	stickyZero, e := z.IsZero(&s.sticky)
	if e != nil {
		return e
	}
	if !stickyZero {
		even, e := z.IsEven(&out.significand)
		if e != nil {
			return e
		}
		if even {
			one := NewInt(1)
			if e := z.addSubWithAllocationContext(&out.significand, &one, &out.significand, subtract, context); e != nil {
				return e
			}
		}
	}
	if e := m.traceFloatDebugIntChecked("add_sub", "sum[-2:sbits+2] = ", &out.significand, 571); e != nil {
		return e
	}
	zero, e := z.IsZero(&out.significand)
	if e != nil {
		return e
	}
	if zero {
		return m.MkZero(out.EBits(), out.SBits(), mode == ArbitraryFloatTowardNegative, out)
	}
	negative, e := z.IsNeg(&out.significand)
	if e != nil {
		return e
	}
	negativeDigit := int64(0)
	if negative {
		negativeDigit = 1
	}
	m.traceFloatDebugSigned("add_sub", "NEG=", negativeDigit, 577)
	if e := z.Abs(&out.significand); e != nil {
		return e
	}
	if e := m.traceFloatDebugIntChecked("add_sub", "fs[-1:sbits+2] = ", &out.significand, 579); e != nil {
		return e
	}
	out.assignSign(!s.a.Sign() && s.b.Sign() && negative || s.a.Sign() && !s.b.Sign() && !negative || s.a.Sign() && s.b.Sign())
	out.exponent = s.a.exponent
	return m.roundWithParentContext(mode, out, &s.roundReceipt, context)
}
