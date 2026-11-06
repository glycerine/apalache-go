package numeral

// Source: src/util/mpf.cpp:1005-1045; src/util/mpf.h:131.

func (m *ArbitraryFloatManager) Sqrt(mode ArbitraryFloatRoundingMode, x, out *ArbitraryFloat) (cause error) {
	for _, v := range [...]*ArbitraryFloat{x, out} {
		if e := m.valueGuard(v); e != nil {
			return e
		}
	}
	if m.sqrt.active {
		return ErrNumericPending
	}
	if e := mpfSourceAssertion(x.EBits() > 0 && x.SBits() > 0, 1006, "x.ebits > 0 && x.sbits > 0"); e != nil {
		return e
	}
	if e := m.traceFloatDebugValueChecked("sqrt", "X = ", x, 1008); e != nil {
		return e
	}
	defer func() {
		if cause == nil {
			cause = m.traceFloatDebugValueChecked("sqrt", "SQRT = ", out, 1044)
		}
	}()
	nan, e := m.IsNaN(x)
	if e != nil {
		return e
	}
	if nan {
		return m.MkNaN(x.EBits(), x.SBits(), out, nil)
	}
	inf, e := m.IsPositiveInf(x)
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
		return m.Copy(out, x)
	}
	if x.Sign() {
		return m.MkNaN(x.EBits(), x.SBits(), out, nil)
	}
	out.assignEbits(x.EBits())
	out.assignSbits(x.SBits())
	out.assignSign(false)
	s := &m.sqrt
	if s.invocation == ^uint64(0) {
		return ErrNumericPending
	}
	*s = arbitraryFloatSqrtScope{manager: m, invocation: s.invocation + 1, active: true, executing: true, aLive: true}
	s.hooks.Policy = s
	if e := s.bind(); e != nil {
		s.executing = false
		return e
	}
	defer func() {
		s.executing = false
		s.bodyCause = cause
		cause = JoinNumericErrors(cause, m.finishSqrtSource())
		cause = JoinNumericErrors(cause, s.unbind())
	}()
	context := s.context()
	z := m.integer
	if e := m.copyWithValueReceiptAndContext(&s.a, x, &s.copyA, context); e != nil {
		return e
	}
	if e := m.unpackRationalNumeral(&s.a, true, context); e != nil {
		return e
	}
	if e := m.traceFloatDebugValueChecked("sqrt", "A = ", &s.a, 1027); e != nil {
		return e
	}
	extra := uint32(7)
	if s.a.exponent%2 != 0 {
		extra = 6
	}
	if e := z.check(&s.a.significand); e != nil {
		return e
	}
	if e := z.mul2kCoreWithAllocationContext(&s.a.significand, x.SBits()+extra, context); e != nil {
		return e
	}
	exact, e := z.rootWithAllocationContext(&s.a.significand, 2, &out.significand, &s.rootCopy, &s.root, context)
	if e != nil {
		return e
	}
	if !exact {
		even, e := z.IsEven(&out.significand)
		if e != nil {
			return e
		}
		if even {
			one := NewInt(1)
			if e := z.addSubWithAllocationContext(&out.significand, &one, &out.significand, true, context); e != nil {
				return e
			}
		}
		if e := m.traceFloatDebugIntChecked("sqrt", "dec'ed ", &out.significand, 1036); e != nil {
			return e
		}
	}
	out.exponent = s.a.exponent >> 1
	if s.a.exponent%2 == 0 {
		out.exponent--
	}
	return m.roundWithParentContext(mode, out, &s.roundReceipt, context)
}
