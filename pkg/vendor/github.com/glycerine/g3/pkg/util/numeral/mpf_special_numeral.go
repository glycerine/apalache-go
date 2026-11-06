package numeral

// Source: src/util/mpf.cpp:1857-1891,1898-1919.

// Each bitfield assignment is a literal source action. The final sign write in
// zero/NaN intentionally occurs after the numerical SET/decrement succeeds.
func (a *ArbitraryFloat) assignSbits(s uint32) { a.format = a.format&0x80007fff | (s&0xffff)<<15 }
func (a *ArbitraryFloat) assignEbits(e uint32) { a.format = a.format&0xffff8000 | e&0x7fff }
func (a *ArbitraryFloat) assignSign(sign bool) {
	a.format &= 0x7fffffff
	if sign {
		a.format |= 0x80000000
	}
}

func (m *ArbitraryFloatManager) MkZero(ebits, sbits uint32, sign bool, out *ArbitraryFloat) error {
	if e := m.valueGuard(out); e != nil {
		return e
	}
	out.assignSbits(sbits)
	out.assignEbits(ebits)
	bottom, e := m.bottomExponent(ebits)
	if e != nil {
		return e
	}
	out.exponent = bottom
	m.valueExecuting = true
	defer func() { m.valueExecuting = false }()
	if e = m.integer.SetInt32(&out.significand, 0); e != nil {
		return e
	}
	out.assignSign(sign)
	return nil
}
func (m *ArbitraryFloatManager) MkInf(ebits, sbits uint32, sign bool, out *ArbitraryFloat) error {
	if e := m.valueGuard(out); e != nil {
		return e
	}
	out.assignSbits(sbits)
	out.assignEbits(ebits)
	out.assignSign(sign)
	top, e := m.topExponent(ebits)
	if e != nil {
		return e
	}
	out.exponent = top
	m.valueExecuting = true
	defer func() { m.valueExecuting = false }()
	return m.integer.SetInt32(&out.significand, 0)
}

// MkMaxValue publishes format, sign and exponent before copying the cached
// fraction. The optional receipt names only the actual allocating SET child.
func (m *ArbitraryFloatManager) MkMaxValue(ebits, sbits uint32, sign bool, out *ArbitraryFloat, receipt *ArbitraryFloatValueCopyReceipt) (cause error) {
	if e := m.valueGuard(out); e != nil {
		return e
	}
	out.assignSbits(sbits)
	out.assignEbits(ebits)
	out.assignSign(sign)
	top, e := m.topExponent(ebits)
	if e != nil {
		return e
	}
	exponent, e := roundSignedSub(top, 1)
	if e != nil {
		return e
	}
	out.exponent = exponent
	w, e := m.powerTwoMinusOneWrapper(sbits-1, false)
	if e != nil {
		return e
	}
	source, e := w.Capture()
	if e != nil {
		return e
	}
	m.valueExecuting = true
	defer func() { m.valueExecuting = false }()
	if e := m.beginValueCopyBinding(receipt, m.integer); e != nil {
		return e
	}
	defer func() { m.finishValueCopyBinding(cause) }()
	return m.copyValueSignificand(&out.significand, &source)
}
func (m *ArbitraryFloatManager) MkNaN(ebits, sbits uint32, out *ArbitraryFloat, receipt *ArbitraryFloatValueCopyReceipt) error {
	return m.mkNaNWithSourceContext(ebits, sbits, out, receipt, integerAllocationInvocationContext{})
}
func (m *ArbitraryFloatManager) mkNaNWithSourceContext(ebits, sbits uint32, out *ArbitraryFloat, receipt *ArbitraryFloatValueCopyReceipt, c integerAllocationInvocationContext) (cause error) {
	if e := m.valueGuard(out); e != nil {
		return e
	}
	out.assignSbits(sbits)
	out.assignEbits(ebits)
	top, e := m.topExponent(ebits)
	if e != nil {
		return e
	}
	out.exponent = top
	power, e := m.powerTwoWrapper(sbits-1, false)
	if e != nil {
		return e
	}
	source, e := power.Capture()
	if e != nil {
		return e
	}
	m.valueExecuting = true
	defer func() { m.valueExecuting = false }()
	if e = m.beginValueCopyBinding(receipt, m.integer); e != nil {
		return e
	}
	m.valueBinding.conversionContext = c
	defer func() { m.finishValueCopyBinding(cause) }()
	if e = m.copyValueSignificand(&out.significand, &source); e != nil {
		return e
	}
	if e = m.integer.Dec(&out.significand); e != nil {
		return e
	}
	out.assignSign(false)
	return nil
}
