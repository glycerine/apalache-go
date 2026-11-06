package numeral

// Source: src/util/mpf.cpp:392-417,1771-1792,1821-1847.

// MkTopExp and MkBotExp are the reserved exponents for special values.
// MkMinExp and MkMaxExp delimit the ordinary exponent range.
// Source: src/util/mpf.cpp:1828-1848; src/util/mpf.h:202-205.
func (m *ArbitraryFloatManager) MkTopExp(ebits uint32) (int64, error) {
	if e := m.valueOwnerGuard(); e != nil {
		return 0, e
	}
	return m.topExponent(ebits)
}
func (m *ArbitraryFloatManager) MkBotExp(ebits uint32) (int64, error) {
	if e := m.valueOwnerGuard(); e != nil {
		return 0, e
	}
	return m.bottomExponent(ebits)
}
func (m *ArbitraryFloatManager) MkMinExp(ebits uint32) (int64, error) {
	if e := m.valueOwnerGuard(); e != nil {
		return 0, e
	}
	return m.minExponent(ebits)
}
func (m *ArbitraryFloatManager) MkMaxExp(ebits uint32) (int64, error) {
	if e := m.valueOwnerGuard(); e != nil {
		return 0, e
	}
	return m.maxExponent(ebits)
}

func (m *ArbitraryFloatManager) topExponent(e uint32) (int64, error) {
	if assertion := mpfSourceAssertion(e >= 2, 1835, "ebits >= 2"); assertion != nil {
		return 0, assertion
	}
	w, err := m.powerTwoWrapper(e-1, false)
	if err != nil {
		return 0, err
	}
	a, err := w.Capture()
	if err != nil {
		return 0, err
	}
	return m.integer.Int64(&a)
}
func (m *ArbitraryFloatManager) bottomExponent(e uint32) (int64, error) {
	if assertion := mpfSourceAssertion(e >= 2, 1830, "ebits >= 2"); assertion != nil {
		return 0, assertion
	}
	w, err := m.powerTwoMinusOneWrapper(e-1, true)
	if err != nil {
		return 0, err
	}
	a, err := w.Capture()
	if err != nil {
		return 0, err
	}
	return m.integer.Int64(&a)
}
func (m *ArbitraryFloatManager) hasTopExponent(a *ArbitraryFloat) (bool, error) {
	if e := m.valueGuard(a); e != nil {
		return false, e
	}
	top, e := m.topExponent(a.EBits())
	if e != nil {
		return false, e
	}
	return a.exponent == top, nil
}
func (m *ArbitraryFloatManager) hasBottomExponent(a *ArbitraryFloat) (bool, error) {
	if e := m.valueGuard(a); e != nil {
		return false, e
	}
	bottom, e := m.bottomExponent(a.EBits())
	if e != nil {
		return false, e
	}
	return a.exponent == bottom, nil
}
func (m *ArbitraryFloatManager) IsNaN(a *ArbitraryFloat) (bool, error) {
	top, e := m.hasTopExponent(a)
	if e != nil || !top {
		return false, e
	}
	zero, e := m.integer.IsZero(&a.significand)
	if e != nil {
		return false, e
	}
	return !zero, nil
}
func (m *ArbitraryFloatManager) IsInf(a *ArbitraryFloat) (bool, error) {
	top, e := m.hasTopExponent(a)
	if e != nil || !top {
		return false, e
	}
	return m.integer.IsZero(&a.significand)
}
func (m *ArbitraryFloatManager) IsZero(a *ArbitraryFloat) (bool, error) {
	bottom, e := m.hasBottomExponent(a)
	if e != nil || !bottom {
		return false, e
	}
	return m.integer.IsZero(&a.significand)
}
func (m *ArbitraryFloatManager) IsOne(a *ArbitraryFloat) (bool, error) {
	if e := m.valueGuard(a); e != nil {
		return false, e
	}
	zero, e := m.integer.IsZero(&a.significand)
	if e != nil || !zero {
		return false, e
	}
	return a.exponent == 0, nil
}
func (m *ArbitraryFloatManager) IsDenormal(a *ArbitraryFloat) (bool, error) {
	zero, e := m.IsZero(a)
	if e != nil || zero {
		return false, e
	}
	return m.hasBottomExponent(a)
}
func (m *ArbitraryFloatManager) IsNormal(a *ArbitraryFloat) (bool, error) {
	top, e := m.hasTopExponent(a)
	if e != nil || top {
		return false, e
	}
	denormal, e := m.IsDenormal(a)
	if e != nil || denormal {
		return false, e
	}
	zero, e := m.IsZero(a)
	if e != nil {
		return false, e
	}
	return !zero, nil
}
func (m *ArbitraryFloatManager) IsPositiveInf(a *ArbitraryFloat) (bool, error) {
	if e := m.valueGuard(a); e != nil {
		return false, e
	}
	if a.Sign() {
		return false, nil
	}
	return m.IsInf(a)
}
func (m *ArbitraryFloatManager) IsNegativeInf(a *ArbitraryFloat) (bool, error) {
	if e := m.valueGuard(a); e != nil {
		return false, e
	}
	if !a.Sign() {
		return false, nil
	}
	return m.IsInf(a)
}
func (m *ArbitraryFloatManager) IsNegative(a *ArbitraryFloat) (bool, error) {
	if e := m.valueGuard(a); e != nil {
		return false, e
	}
	if !a.Sign() {
		return false, nil
	}
	nan, e := m.IsNaN(a)
	if e != nil {
		return false, e
	}
	return !nan, nil
}
func (m *ArbitraryFloatManager) IsPositive(a *ArbitraryFloat) (bool, error) {
	if e := m.valueGuard(a); e != nil {
		return false, e
	}
	if a.Sign() {
		return false, nil
	}
	nan, e := m.IsNaN(a)
	if e != nil {
		return false, e
	}
	return !nan, nil
}
func (m *ArbitraryFloatManager) IsNegativeZero(a *ArbitraryFloat) (bool, error) {
	if e := m.valueGuard(a); e != nil {
		return false, e
	}
	if !a.Sign() {
		return false, nil
	}
	return m.IsZero(a)
}
func (m *ArbitraryFloatManager) IsPositiveZero(a *ArbitraryFloat) (bool, error) {
	if e := m.valueGuard(a); e != nil {
		return false, e
	}
	if a.Sign() {
		return false, nil
	}
	return m.IsZero(a)
}

// Source: src/util/mpf.cpp:1840-1848. These bounds have their own ebits>0
// assertion; reusing bottomExponent would incorrectly reject ebits=1.
func (m *ArbitraryFloatManager) minExponent(e uint32) (int64, error) {
	if assertion := mpfSourceAssertion(e > 0, 1840, "ebits > 0"); assertion != nil {
		return 0, assertion
	}
	w, err := m.powerTwoMinusOneWrapper(e-1, true)
	if err != nil {
		return 0, err
	}
	a, err := w.Capture()
	if err != nil {
		return 0, err
	}
	r, err := m.integer.Int64(&a)
	if err != nil {
		return 0, err
	}
	return r + 1, nil
}
func (m *ArbitraryFloatManager) maxExponent(e uint32) (int64, error) {
	if assertion := mpfSourceAssertion(e > 0, 1846, "ebits > 0"); assertion != nil {
		return 0, assertion
	}
	w, err := m.powerTwoMinusOneWrapper(e-1, false)
	if err != nil {
		return 0, err
	}
	a, err := w.Capture()
	if err != nil {
		return 0, err
	}
	return m.integer.Int64(&a)
}
