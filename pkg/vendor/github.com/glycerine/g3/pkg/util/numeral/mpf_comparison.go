package numeral

// Source: src/util/mpf.cpp:416-469; src/util/mpf.h:116-123.

func (m *ArbitraryFloatManager) comparisonGuard(x, y *ArbitraryFloat, line int32) error {
	if e := m.valueGuard(x); e != nil {
		return e
	}
	if e := m.valueGuard(y); e != nil {
		return e
	}
	return mpfSourceAssertion(x.SBits() == y.SBits() && x.EBits() == y.EBits(), line, "x.sbits == y.sbits && x.ebits == y.ebits")
}
func (m *ArbitraryFloatManager) eitherNaN(x, y *ArbitraryFloat) (bool, error) {
	nan, e := m.IsNaN(x)
	if e != nil || nan {
		return nan, e
	}
	return m.IsNaN(y)
}
func (m *ArbitraryFloatManager) bothZero(x, y *ArbitraryFloat) (bool, error) {
	zero, e := m.IsZero(x)
	if e != nil || !zero {
		return false, e
	}
	return m.IsZero(y)
}
func (m *ArbitraryFloatManager) Eq(x, y *ArbitraryFloat) (bool, error) {
	if e := m.comparisonGuard(x, y, 417); e != nil {
		return false, e
	}
	if nan, e := m.eitherNaN(x, y); e != nil || nan {
		return false, e
	}
	if zero, e := m.bothZero(x, y); e != nil || zero {
		return zero, e
	}
	if x.Sign() != y.Sign() || x.Exponent() != y.Exponent() {
		return false, nil
	}
	c, e := m.integer.Compare(&x.significand, &y.significand)
	return c == 0, e
}
func (m *ArbitraryFloatManager) Lt(x, y *ArbitraryFloat) (bool, error) {
	if e := m.comparisonGuard(x, y, 429); e != nil {
		return false, e
	}
	if nan, e := m.eitherNaN(x, y); e != nil || nan {
		return false, e
	}
	if zero, e := m.bothZero(x, y); e != nil || zero {
		return false, e
	}
	if x.Sign() {
		if !y.Sign() {
			return true, nil
		}
		if y.Exponent() != x.Exponent() {
			return y.Exponent() < x.Exponent(), nil
		}
		c, e := m.integer.Compare(&y.significand, &x.significand)
		return c < 0, e
	}
	if y.Sign() {
		return false, nil
	}
	if x.Exponent() != y.Exponent() {
		return x.Exponent() < y.Exponent(), nil
	}
	c, e := m.integer.Compare(&x.significand, &y.significand)
	return c < 0, e
}
func (m *ArbitraryFloatManager) Lte(x, y *ArbitraryFloat) (bool, error) {
	if e := m.comparisonGuard(x, y, 451); e != nil {
		return false, e
	}
	less, e := m.Lt(x, y)
	if e != nil || less {
		return less, e
	}
	return m.Eq(x, y)
}
func (m *ArbitraryFloatManager) Gt(x, y *ArbitraryFloat) (bool, error) {
	if e := m.comparisonGuard(x, y, 456); e != nil {
		return false, e
	}
	if nan, e := m.eitherNaN(x, y); e != nil || nan {
		return false, e
	}
	if zero, e := m.bothZero(x, y); e != nil || zero {
		return false, e
	}
	lessEqual, e := m.Lte(x, y)
	if e != nil {
		return false, e
	}
	return !lessEqual, nil
}
func (m *ArbitraryFloatManager) Gte(x, y *ArbitraryFloat) (bool, error) {
	if e := m.comparisonGuard(x, y, 466); e != nil {
		return false, e
	}
	greater, e := m.Gt(x, y)
	if e != nil || greater {
		return greater, e
	}
	return m.Eq(x, y)
}
