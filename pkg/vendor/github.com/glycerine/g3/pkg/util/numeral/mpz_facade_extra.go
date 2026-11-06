package numeral

// Concrete public-method closure of src/util/mpz.h, internal 32-bit digits.

// Sign is the source m_val: full inline int32, or -1/+1 for a big numeral.
func (m *IntegerManager) Sign(a *Int) (int32, error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	return a.Val, nil
}
func (m *IntegerManager) IsPos(a *Int) (bool, error) {
	v, e := m.Sign(a)
	if e != nil {
		return false, e
	}
	return v > 0, nil
}
func (m *IntegerManager) IsNeg(a *Int) (bool, error) {
	v, e := m.Sign(a)
	if e != nil {
		return false, e
	}
	return v < 0, nil
}
func (m *IntegerManager) IsZero(a *Int) (bool, error) {
	v, e := m.Sign(a)
	if e != nil {
		return false, e
	}
	return v == 0, nil
}
func (m *IntegerManager) IsNonPos(a *Int) (bool, error) {
	v, e := m.IsPos(a)
	if e != nil {
		return false, e
	}
	return !v, nil
}
func (m *IntegerManager) IsNonNeg(a *Int) (bool, error) {
	v, e := m.IsNeg(a)
	if e != nil {
		return false, e
	}
	return !v, nil
}
func (m *IntegerManager) IsOne(a *Int) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	return a.IsSmall() && a.Val == 1, nil
}
func (m *IntegerManager) IsMinusOne(a *Int) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	return a.IsSmall() && a.Val == -1, nil
}

// Source inc and dec both call add, with inline +1 and -1 respectively.
func (m *IntegerManager) Inc(a *Int) error { one := NewInt(1); return m.Add(a, &one, a) }
func (m *IntegerManager) Dec(a *Int) error { minusOne := NewInt(-1); return m.Add(a, &minusOne, a) }
func (m *IntegerManager) Equal(a, b *Int) (bool, error) {
	v, e := m.Compare(a, b)
	if e != nil {
		return false, e
	}
	return v == 0, nil
}
func (m *IntegerManager) NotEqual(a, b *Int) (bool, error) {
	v, e := m.Equal(a, b)
	if e != nil {
		return false, e
	}
	return !v, nil
}
func (m *IntegerManager) Less(a, b *Int) (bool, error) {
	v, e := m.Compare(a, b)
	if e != nil {
		return false, e
	}
	return v < 0, nil
}
func (m *IntegerManager) Greater(a, b *Int) (bool, error) { return m.Less(b, a) }
func (m *IntegerManager) GreaterEqual(a, b *Int) (bool, error) {
	v, e := m.Less(a, b)
	if e != nil {
		return false, e
	}
	return !v, nil
}
func (m *IntegerManager) LessEqual(a, b *Int) (bool, error) {
	v, e := m.Less(b, a)
	if e != nil {
		return false, e
	}
	return !v, nil
}
func (m *IntegerManager) LessInt(a *Int, b int32) (bool, error) {
	if e := m.check(a); e != nil {
		return false, e
	}
	if a.IsSmall() {
		return a.Val < b, nil
	}
	small := NewInt(b)
	return m.Less(a, &small)
}

func (m *IntegerManager) SetInt32(a *Int, n int32) error {
	if e := m.Del(a); e != nil {
		return e
	}
	a.Val = n
	return nil
}

// Unlike set(uint64), the source unsigned-int overload includes INT_MAX in
// its inline range. Large unsigned-int values use signed int64 construction.
func (m *IntegerManager) SetUint32(a *Int, n uint32) error {
	return m.setUint32WithAllocationContext(a, n, integerAllocationInvocationContext{})
}
func (m *IntegerManager) setUint32WithAllocationContext(a *Int, n uint32, context integerAllocationInvocationContext) error {
	if n <= 2147483647 {
		return m.SetInt32(a, int32(n))
	}
	return m.setInt64WithAllocationContext(a, int64(uint64(n)), context)
}

// Dup creates an independent owning numeral. Del the returned Int explicitly;
// do not shallow-copy it when big. Copy preserves source requested capacity.
func (m *IntegerManager) Dup(a *Int) (Int, error) {
	var out Int
	if e := m.Copy(&out, a); e != nil {
		m.Del(&out)
		return Int{}, e
	}
	return out, nil
}
