package numeral

// Source: src/util/mpfx.cpp:864-868 and src/util/mpfx.h:384.

import "math"

// PrevPowerOfTwo returns the exponent of the highest set bit in the integer
// part. The source unsigned subtraction wraps for a positive value below one;
// the retained nth-root caller only invokes this method for values at least one.
func (m *FixedPointManager) PrevPowerOfTwo(a *FixedPoint) (uint32, error) {
	if err := m.check(a); err != nil {
		return 0, err
	}
	if a.scaled.Sign() <= 0 {
		return 0, nil
	}
	integerBits := a.scaled.BitLen() - int(m.fracBits)
	if integerBits <= 0 {
		return math.MaxUint32, nil
	}
	return uint32(integerBits - 1), nil
}
