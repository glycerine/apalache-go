package numeral

// Concrete indexed bit_util and source MPN Algorithms A/S/M, no borrowed slice.

import (
	"math/bits"
)

type ffLimbView struct {
	external     []uint32
	m            *FastFloatManager
	v            *[]uint32
	offset, size uint32
	literal      uint32
}

func (v ffLimbView) read(i uint32) uint32 {
	if v.external != nil {
		return v.external[v.offset+i]
	}
	if v.v == nil {
		if i == 0 {
			return v.literal
		}
		return 0
	}
	return v.m.read(v.v, v.offset+i)
}
func (v ffLimbView) write(i, n uint32) {
	if v.external != nil {
		v.external[v.offset+i] = n
		return
	}
	v.m.write(v.v, v.offset+i, n)
}
func ffLeading(n uint32) uint32 { return uint32(bits.LeadingZeros32(n)) }
func ffNLZ(v ffLimbView) uint32 {
	n := uint32(0)
	for i := v.size; i > 0; {
		i--
		w := v.read(i)
		if w != 0 {
			return n + ffLeading(w)
		}
		n += 32
	}
	return n
}
func ffNTZ(v ffLimbView) uint32 {
	n := uint32(0)
	for i := uint32(0); i < v.size; i++ {
		w := v.read(i)
		if w != 0 {
			return n + uint32(bits.TrailingZeros32(w))
		}
		n += 32
	}
	return n
}
func ffZero(v ffLimbView) {
	for i := uint32(0); i < v.size; i++ {
		v.write(i, 0)
	}
}
func ffIsZero(v ffLimbView) bool {
	for i := uint32(0); i < v.size; i++ {
		if v.read(i) != 0 {
			return false
		}
	}
	return true
}
func ffHasLow(v ffLimbView, k uint32) bool {
	words := min(k/32, v.size)
	for i := uint32(0); i < words; i++ {
		if v.read(i) != 0 {
			return true
		}
	}
	return words < v.size && v.read(words)&((uint32(1)<<(k%32))-1) != 0
}
func ffCopy(a, b ffLimbView) {
	for i := uint32(0); i < b.size; i++ {
		w := uint32(0)
		if i < a.size {
			w = a.read(i)
		}
		b.write(i, w)
	}
}
func ffCompare(a, b ffLimbView) int {
	for i := max(a.size, b.size); i > 0; {
		i--
		u, v := uint32(0), uint32(0)
		if i < a.size {
			u = a.read(i)
		}
		if i < b.size {
			v = b.read(i)
		}
		if u < v {
			return -1
		}
		if u > v {
			return 1
		}
	}
	return 0
}
func ffInc(v ffLimbView) bool {
	for i := uint32(0); i < v.size; i++ {
		w := v.read(i) + 1
		v.write(i, w)
		if w != 0 {
			return true
		}
	}
	return false
}
func ffShift(a, b ffLimbView, k uint32, left bool) {
	word, bit := k/32, k%32
	if left {
		for i := b.size; i > 0; {
			i--
			w := uint32(0)
			if i >= word {
				j := i - word
				if j < a.size {
					w = a.read(j) << bit
				}
				if bit != 0 && j > 0 && j-1 < a.size {
					w |= a.read(j-1) >> (32 - bit)
				}
			}
			b.write(i, w)
		}
	} else {
		for i := uint32(0); i < b.size; i++ {
			w := uint32(0)
			j := uint64(i) + uint64(word)
			if j < uint64(a.size) {
				w = a.read(uint32(j)) >> bit
				if bit != 0 && j+1 < uint64(a.size) {
					w |= a.read(uint32(j+1)) << (32 - bit)
				}
			}
			b.write(i, w)
		}
	}
}
func ffAdd(a, b, c ffLimbView) {
	carry := uint64(0)
	for i := uint32(0); i < c.size; i++ {
		n := carry
		if i < a.size {
			n += uint64(a.read(i))
		}
		if i < b.size {
			n += uint64(b.read(i))
		}
		c.write(i, uint32(n))
		carry = n >> 32
	}
}
func ffSub(a, b, c ffLimbView) uint32 {
	borrow := uint64(0)
	for i := uint32(0); i < c.size; i++ {
		u, v := uint32(0), uint32(0)
		if i < a.size {
			u = a.read(i)
		}
		if i < b.size {
			v = b.read(i)
		}
		rhs := uint64(v) + borrow
		c.write(i, uint32(uint64(u)-rhs))
		borrow = 0
		if uint64(u) < rhs {
			borrow = 1
		}
	}
	return uint32(borrow)
}
func ffMul(a, b, c ffLimbView) {
	ffZero(c)
	for j := uint32(0); j < b.size; j++ {
		carry := uint64(0)
		v := b.read(j)
		for i := uint32(0); i < a.size; i++ {
			n := uint64(a.read(i))*uint64(v) + uint64(c.read(i+j)) + carry
			c.write(i+j, uint32(n))
			carry = n >> 32
		}
		c.write(j+a.size, uint32(carry))
	}
}
