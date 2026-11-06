package numeral

// Source: src/util/f2n.h:57-69,115-149, instantiated as f2n<mpf_manager>.

func (m *ArbitraryFloatNumeralManager) IsZero(x *ArbitraryFloat) (bool, error) {
	if e := m.guard(); e != nil {
		return false, e
	}
	return m.core.IsZero(x)
}
func (m *ArbitraryFloatNumeralManager) IsNeg(x *ArbitraryFloat) (bool, error) {
	if e := m.guard(); e != nil {
		return false, e
	}
	neg, e := m.core.IsNegative(x)
	if e != nil || !neg {
		return neg, e
	}
	zero, e := m.core.IsZero(x)
	if e != nil {
		return false, e
	}
	return !zero, nil
}
func (m *ArbitraryFloatNumeralManager) IsPos(x *ArbitraryFloat) (bool, error) {
	if e := m.guard(); e != nil {
		return false, e
	}
	pos, e := m.core.IsPositive(x)
	if e != nil || !pos {
		return pos, e
	}
	zero, e := m.core.IsZero(x)
	if e != nil {
		return false, e
	}
	return !zero, nil
}
func (m *ArbitraryFloatNumeralManager) IsNonNeg(x *ArbitraryFloat) (bool, error) {
	neg, e := m.IsNeg(x)
	if e != nil {
		return false, e
	}
	return !neg, nil
}
func (m *ArbitraryFloatNumeralManager) IsNonPos(x *ArbitraryFloat) (bool, error) {
	pos, e := m.IsPos(x)
	if e != nil {
		return false, e
	}
	return !pos, nil
}
func (m *ArbitraryFloatNumeralManager) IsOne(x *ArbitraryFloat) (bool, error) {
	if e := m.guard(); e != nil {
		return false, e
	}
	return m.core.IsOne(x)
}
func (m *ArbitraryFloatNumeralManager) IsInteger(x *ArbitraryFloat) (bool, error) {
	if e := m.guard(); e != nil {
		return false, e
	}
	return m.core.IsInt(x)
}
func (m *ArbitraryFloatNumeralManager) IsMinusOne(x *ArbitraryFloat) (bool, error) {
	if e := m.guard(); e != nil {
		return false, e
	}
	if e := m.core.Neg(x); e != nil {
		return false, e
	}
	one, e := m.core.IsOne(x)
	if e != nil {
		return false, e
	} // source does not restore after a thrown query
	return one, m.core.Neg(x)
}
func (m *ArbitraryFloatNumeralManager) Equal(x, y *ArbitraryFloat) (bool, error) {
	if e := m.guard(); e != nil {
		return false, e
	}
	return m.core.Eq(x, y)
}
func (m *ArbitraryFloatNumeralManager) Less(x, y *ArbitraryFloat) (bool, error) {
	if e := m.guard(); e != nil {
		return false, e
	}
	return m.core.Lt(x, y)
}
func (m *ArbitraryFloatNumeralManager) LessEqual(x, y *ArbitraryFloat) (bool, error) {
	if e := m.guard(); e != nil {
		return false, e
	}
	return m.core.Lte(x, y)
}
func (m *ArbitraryFloatNumeralManager) Greater(x, y *ArbitraryFloat) (bool, error) {
	if e := m.guard(); e != nil {
		return false, e
	}
	return m.core.Gt(x, y)
}
func (m *ArbitraryFloatNumeralManager) GreaterEqual(x, y *ArbitraryFloat) (bool, error) {
	if e := m.guard(); e != nil {
		return false, e
	}
	return m.core.Gte(x, y)
}
func (m *ArbitraryFloatNumeralManager) Abs(in, out *ArbitraryFloat) error {
	if e := m.guard(); e != nil {
		return e
	}
	if in == out {
		return m.core.Abs(out)
	}
	return m.core.AbsCopy(out, in, nil)
}
func (m *ArbitraryFloatNumeralManager) Neg(in, out *ArbitraryFloat) error {
	if e := m.guard(); e != nil {
		return e
	}
	if in == out {
		return m.core.Neg(out)
	}
	return m.core.NegCopy(out, in, nil)
}

func (m *ArbitraryFloatNumeralManager) Floor(in, out *ArbitraryFloat) error {
	if e := m.check(in); e != nil {
		return e
	}
	if e := m.core.RoundToIntegral(ArbitraryFloatTowardNegative, in, &m.tmp); e != nil {
		return e
	}
	if e := m.check(&m.tmp); e != nil {
		return e
	}
	le, e := m.core.Lte(&m.tmp, in)
	if e != nil {
		return e
	}
	if le {
		if e := m.core.Copy(out, &m.tmp); e != nil {
			return e
		}
	} else {
		if e := m.core.Sub(ArbitraryFloatTowardNegative, &m.tmp, &m.one, out); e != nil {
			return e
		}
	}
	return m.check(out)
}
func (m *ArbitraryFloatNumeralManager) Ceil(in, out *ArbitraryFloat) error {
	if e := m.check(in); e != nil {
		return e
	}
	if e := m.core.RoundToIntegral(ArbitraryFloatTowardPositive, in, &m.tmp); e != nil {
		return e
	}
	if e := m.check(&m.tmp); e != nil {
		return e
	}
	ge, e := m.core.Gte(&m.tmp, in)
	if e != nil {
		return e
	}
	if ge {
		if e := m.core.Copy(out, &m.tmp); e != nil {
			return e
		}
	} else {
		if e := m.core.Add(ArbitraryFloatTowardNegative, &m.tmp, &m.one, out); e != nil {
			return e
		}
	}
	return m.check(out)
}
