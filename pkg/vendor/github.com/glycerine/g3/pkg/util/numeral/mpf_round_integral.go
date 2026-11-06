package numeral

// Source: src/util/mpf.cpp:1047-1170; src/util/mpf.h:133.

func (m *ArbitraryFloatManager) RoundToIntegral(mode ArbitraryFloatRoundingMode, x, out *ArbitraryFloat) (cause error) {
	if e := m.valueGuard(x); e != nil {
		return e
	}
	if e := m.valueGuard(out); e != nil {
		return e
	}
	if m.integralRound.active {
		return ErrNumericPending
	}
	if e := mpfSourceAssertion(x.EBits() > 0 && x.SBits() > 0, 1048, "x.ebits > 0 && x.sbits > 0"); e != nil {
		return e
	}
	if e := m.traceFloatDebugValueChecked("round_to_integral", "X = ", x, 1050); e != nil {
		return e
	}
	defer func() {
		if cause == nil {
			cause = m.traceFloatDebugValueChecked("round_to_integral", "INTEGRAL = ", out, 1167)
		}
	}()
	nan, e := m.IsNaN(x)
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
		return m.Copy(out, x)
	}
	zero, e := m.IsZero(x)
	if e != nil {
		return e
	}
	if zero {
		return m.MkZero(x.EBits(), x.SBits(), x.Sign(), out)
	}
	if x.exponent < 0 {
		switch mode {
		case ArbitraryFloatTowardZero:
			return m.MkZero(x.EBits(), x.SBits(), x.Sign(), out)
		case ArbitraryFloatTowardNegative:
			if x.Sign() {
				return m.MkOne(x.EBits(), x.SBits(), true, out)
			}
			return m.MkZero(x.EBits(), x.SBits(), false, out)
		case ArbitraryFloatTowardPositive:
			if x.Sign() {
				return m.MkZero(x.EBits(), x.SBits(), true, out)
			}
			return m.MkOne(x.EBits(), x.SBits(), false, out)
		default:
			if e := mpfSourceAssertion(mode == ArbitraryFloatNearestEven || mode == ArbitraryFloatNearestAway, 1074, "rm == MPF_ROUND_NEAREST_TEVEN || rm == MPF_ROUND_NEAREST_TAWAY"); e != nil {
				return e
			}
			sigZero, e := m.integer.IsZero(&x.significand)
			if e != nil {
				return e
			}
			tie := sigZero && x.exponent == -1
			tieDigit := int64(0)
			if tie {
				tieDigit = 1
			}
			m.traceFloatDebugSigned("round_to_integral", "tie = ", tieDigit, 1076)
			if tie && mode == ArbitraryFloatNearestEven {
				return m.MkZero(x.EBits(), x.SBits(), x.Sign(), out)
			}
			if tie && mode == ArbitraryFloatNearestAway {
				return m.MkOne(x.EBits(), x.SBits(), x.Sign(), out)
			}
			if x.exponent < -1 {
				return m.MkZero(x.EBits(), x.SBits(), x.Sign(), out)
			}
			return m.MkOne(x.EBits(), x.SBits(), x.Sign(), out)
		}
	}
	if x.exponent >= int64(x.SBits()-1) {
		return m.Copy(out, x)
	}
	if e := mpfSourceAssertion(x.exponent >= 0 && x.exponent < int64(x.SBits()-1), 1090, "x.exponent >= 0 && x.exponent < x.sbits-1"); e != nil {
		return e
	}
	out.assignEbits(x.EBits())
	out.assignSbits(x.SBits())
	out.assignSign(x.Sign())
	s := &m.integralRound
	if s.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	*s = arbitraryFloatIntegralRoundScope{manager: m, invocation: s.invocation + 1, active: true, executing: true, aLive: true}
	s.hooks.Policy = s
	if e := s.bind(); e != nil {
		s.executing = false
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = JoinNumericErrors(cause, m.finishIntegralRoundSource())
		cause = JoinNumericErrors(cause, s.unbind())
	}()
	context := s.context()
	if e := m.copyWithValueReceiptAndContext(&s.a, x, &s.copyA, context); e != nil {
		return e
	}
	if e := m.unpackRationalNumeral(&s.a, true, context); e != nil {
		return e
	}
	if e := m.traceRoundIntegralRawOperand(&s.a); e != nil {
		return e
	}
	if modularSourceDebug {
		enabled, e := mpfSourceAssertionsEnabled()
		if e != nil {
			return e
		}
		if enabled {
			p, e := s.power(x.SBits())
			if e != nil {
				return e
			}
			c, e := m.integer.Compare(&s.a.significand, &p)
			if e != nil {
				return e
			}
			if e := mpfSourceAssertion(c < 0, 1102, "m_mpz_manager.lt(a.significand(), m_powers2(x.sbits))"); e != nil {
				return e
			}
		}
		enabled, e = mpfSourceAssertionsEnabled()
		if e != nil {
			return e
		}
		if enabled {
			p, e := s.power(x.SBits() - 1)
			if e != nil {
				return e
			}
			c, e := m.integer.Compare(&s.a.significand, &p)
			if e != nil {
				return e
			}
			if e := mpfSourceAssertion(c >= 0, 1103, "m_mpz_manager.ge(a.significand(), m_powers2(x.sbits - 1))"); e != nil {
				return e
			}
		}
	}
	out.exponent = s.a.exponent
	if e := m.integer.copyFromOwnerWithAllocationContext(&out.significand, m.integer, &s.a.significand, nil, context); e != nil {
		return e
	}
	shift := out.SBits() - 1 - uint32(out.exponent)
	shiftP, e := s.power(shift)
	if e != nil {
		return e
	}
	shiftM1, e := s.power(shift - 1)
	if e != nil {
		return e
	}
	m.traceFloatDebugSigned("round_to_integral", "shift=", int64(shift), 1111)
	if e := m.traceFloatDebugIntChecked("round_to_integral", "shiftm1_p=", &shiftM1, 1112); e != nil {
		return e
	}
	s.divLive, s.remLive = true, true
	if e := m.integer.machineDivRemWithAllocationContext(&out.significand, &shiftP, &s.div, &s.rem, context); e != nil {
		return e
	}
	if e := m.traceRoundIntegralQuotient(&s.div, &s.rem); e != nil {
		return e
	}
	inc := false
	incKind := uint8(0)
	switch mode {
	case ArbitraryFloatNearestEven, ArbitraryFloatNearestAway:
		c, e := m.integer.Compare(&s.rem, &shiftM1)
		if e != nil {
			return e
		}
		tie := c == 0
		// Retain the source's three separate comparisons, including the
		// less-than read in release where only TRACE consumes its result.
		less, e := m.integer.Less(&s.rem, &shiftM1)
		if e != nil {
			return e
		}
		more, e := m.integer.Greater(&s.rem, &shiftM1)
		if e != nil {
			return e
		}
		m.traceRoundIntegralTie(tie, less, more)
		if tie {
			if mode == ArbitraryFloatNearestEven {
				inc, e = m.integer.IsOdd(&s.div)
				if e != nil {
					return e
				}
			} else {
				inc = true
			}
			if inc {
				incKind = 1
			}
		} else {
			if e := mpfSourceAssertion(less || more, 1134, "less_than_tie || more_than_tie"); e != nil {
				return e
			}
			inc = more
			if inc {
				incKind = 2
			}
		}
	case ArbitraryFloatTowardPositive, ArbitraryFloatTowardNegative:
		isZero, e := m.integer.IsZero(&s.rem)
		if e != nil {
			return e
		}
		inc = !isZero && (mode == ArbitraryFloatTowardNegative && out.Sign() || mode == ArbitraryFloatTowardPositive && !out.Sign())
	}
	if inc {
		if incKind == 1 {
			m.traceFloatDebugText("round_to_integral", "div++ (1)", 1129)
		}
		one := NewInt(1)
		if e := m.integer.addSubWithAllocationContext(&s.div, &one, &s.div, false, context); e != nil {
			return e
		}
		if incKind == 2 {
			m.traceFloatDebugText("round_to_integral", "div++ (2)", 1137)
		}
	}
	if e := m.integer.shiftWithAllocationContext(&s.div, shift, &out.significand, true, context); e != nil {
		return e
	}
	if modularSourceDebug {
		enabled, e := mpfSourceAssertionsEnabled()
		if e != nil {
			return e
		}
		if enabled {
			p, e := s.power(out.SBits() - 1)
			if e != nil {
				return e
			}
			c, e := m.integer.Compare(&out.significand, &p)
			if e != nil {
				return e
			}
			if e := mpfSourceAssertion(c >= 0, 1156, "m_mpz_manager.ge(o.significand, m_powers2(o.sbits - 1))"); e != nil {
				return e
			}
		}
	}
	for {
		p, e := s.power(out.SBits())
		if e != nil {
			return e
		}
		c, e := m.integer.Compare(&out.significand, &p)
		if e != nil {
			return e
		}
		if c < 0 {
			break
		}
		if e := m.integer.MachineDiv2kInPlace(&out.significand, 1); e != nil {
			return e
		}
		out.exponent++
	}
	p, e := s.power(out.SBits() - 1)
	if e != nil {
		return e
	}
	return m.integer.addSubWithAllocationContext(&out.significand, &p, &out.significand, true, context)
}
