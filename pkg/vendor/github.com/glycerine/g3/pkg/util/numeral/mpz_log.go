package numeral

// Source: src/util/mpz.cpp log2/mlog2/bitsize (_MP_INTERNAL 32-bit limbs),
// src/util/mpz.h prev_power_of_two and src/util/util.cpp log2(unsigned).

import (
	"github.com/glycerine/g3/pkg/arena"
)

func log2Uint32(v uint32) uint32 {
	var r uint32
	if v&0xffff0000 != 0 {
		v >>= 16
		r |= 16
	}
	if v&0xff00 != 0 {
		v >>= 8
		r |= 8
	}
	if v&0xf0 != 0 {
		v >>= 4
		r |= 4
	}
	if v&0xc != 0 {
		v >>= 2
		r |= 2
	}
	if v&2 != 0 {
		r |= 1
	}
	return r
}

// Log2 returns the most significant bit position for positive values and zero
// for nonpositive values, as in the source; it is not a signed logarithm.
func (m *IntegerManager) Log2(a *Int) (uint32, error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	if a.Val <= 0 {
		return 0, nil
	}
	return m.magnitudeLog2(a)
}

// MLog2 returns Log2 of the magnitude for negative values, zero otherwise.
func (m *IntegerManager) MLog2(a *Int) (uint32, error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	if a.Val >= 0 {
		return 0, nil
	}
	return m.magnitudeLog2(a)
}
func (m *IntegerManager) magnitudeLog2(a *Int) (uint32, error) {
	if a.IsSmall() {
		v := int64(a.Val)
		if v < 0 {
			v = -v
		}
		return log2Uint32(uint32(v)), nil
	}
	size := m.Cell(a.Cell).Size
	// Check the source unsigned position computation before narrowing.
	k := uint64(size-1)*32 + uint64(log2Uint32(m.Digit(a.Cell, size-1)))
	if k > uint64(^uint32(0)) {
		return 0, arena.ErrBounds
	}
	return uint32(k), nil
}

// Bitsize follows the source statistics contract, which assigns zero size 1.
func (m *IntegerManager) Bitsize(a *Int) (uint32, error) {
	if e := m.check(a); e != nil {
		return 0, e
	}
	var k uint32
	var e error
	if a.Val >= 0 {
		k, e = m.Log2(a)
	} else {
		k, e = m.MLog2(a)
	}
	if e != nil {
		return 0, e
	}
	if k == ^uint32(0) {
		return 0, arena.ErrBounds
	}
	return k + 1, nil
}
func (m *IntegerManager) PrevPowerOfTwo(a *Int) (uint32, error) { return m.Log2(a) }
