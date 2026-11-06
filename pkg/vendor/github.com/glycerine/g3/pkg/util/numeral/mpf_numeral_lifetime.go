package numeral

// Source: src/util/f2n.h:60,82; instantiated as f2n<mpf_manager>.

// The interval engine branches on this static f2n trait.
func (*ArbitraryFloatNumeralManager) Precise() bool { return false }

// ext_numeral's inverse assertion uses this static field trait.
func (*ArbitraryFloatNumeralManager) Field() bool { return true }

// Del releases a caller-owned numeral while the wrapper's borrowed manager
// remains live.
func (m *ArbitraryFloatNumeralManager) Del(x *ArbitraryFloat) error {
	if e := m.guard(); e != nil {
		return e
	}
	return m.core.Del(x)
}

// Swap transfers the numeral values, including any owned significand cell.
// Like the source static swap, it does not copy either owned cell.
func (m *ArbitraryFloatNumeralManager) Swap(x, y *ArbitraryFloat) error {
	if e := m.guard(); e != nil {
		return e
	}
	if x == nil || y == nil {
		return ErrNumericOwner
	}
	*x, *y = *y, *x
	return nil
}

// SetUnsigned follows f2n's unsigned-to-double overload. Every uint32 value
// converts exactly before the underlying MPF setter rounds to its format.
func (m *ArbitraryFloatNumeralManager) SetUnsigned(out *ArbitraryFloat, value uint32) error {
	return m.SetDouble(out, float64(value))
}

func (m *ArbitraryFloatNumeralManager) PrevPowerOfTwo(x *ArbitraryFloat) (uint32, error) {
	if e := m.guard(); e != nil {
		return 0, e
	}
	return m.core.PrevPowerOfTwo(x)
}
