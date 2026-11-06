// Package numeral contains the fork's independent exact numerical algorithms.
package numeral

// Translated from src/util/mpn.h and src/util/mpn.cpp.

func limb(a []uint32, i int) uint32 {
	if i < len(a) {
		return a[i]
	}
	return 0
}

// Compare translates mpn_manager::compare; leading zero limbs are allowed.
func (m *NaturalManager) Compare(a, b []uint32) int {
	m.traceMPNUnary(a, 420, false)
	result := 0
	for j := max(len(a), len(b)) - 1; j >= 0; j-- {
		u, v := limb(a, j), limb(b, j)
		if u > v {
			result = 1
			break
		}
		if u < v {
			result = -1
			break
		}
	}
	m.traceMPNCompareResult(result)
	m.traceMPNUnary(b, 427, true)
	return result
}

// Add translates Knuth's Algorithm A. c must have exactly max lengths + 1
// limbs; at least one input is nonempty. Exact input/output aliasing is allowed;
// shifted overlapping slices are outside the source contract. The returned
// normalized size is at least one, including for zero.
func (m *NaturalManager) Add(a, b, c []uint32) int {
	m.traceMPNBinary(a, b, "+")
	n := max(len(a), len(b))
	if n == 0 || len(c) != n+1 {
		panic("mpnum: invalid addition buffer")
	}
	var k uint32
	for j := 0; j < n; j++ {
		u, v := limb(a, j), limb(b, j)
		r := u + v
		c1 := r < u
		c[j] = r + k
		c2 := c[j] < r
		k = 0
		if c1 || c2 {
			k = 1
		}
	}
	c[n] = k
	size := n + 1
	for size > 1 && c[size-1] == 0 {
		size--
	}
	m.traceMPNUnary(c[:size], 427, true)
	return size
}

// Sub translates Knuth's Algorithm S. c has max input lengths limbs; it
// receives subtraction modulo 2^(32*len(c)). Return 1 denotes a borrow.
// Exact input/output aliasing is allowed, shifted overlap is not.
func (m *NaturalManager) Sub(a, b, c []uint32) uint32 {
	m.traceMPNBinary(a, b, "-")
	n := max(len(a), len(b))
	if len(c) != n {
		panic("mpnum: invalid subtraction buffer")
	}
	var k uint32
	for j := 0; j < n; j++ {
		u, v := limb(a, j), limb(b, j)
		r := u - v
		c1 := r > u
		c[j] = r - k
		c2 := c[j] > r
		k = 0
		if c1 || c2 {
			k = 1
		}
	}
	m.traceMPNUnary(c[:len(a)], 427, true)
	return k
}

// Mul translates Knuth's Algorithm M. c has len(a)+len(b) limbs and must
// not overlap either input (mpz handles its own aliasing through scratch).
func (m *NaturalManager) Mul(a, b, c []uint32) {
	m.traceMPNBinary(a, b, "*")
	if len(c) != len(a)+len(b) {
		panic("mpnum: invalid multiplication buffer")
	}
	for i := range a {
		c[i] = 0
	}
	for j, v := range b {
		if v == 0 {
			c[j+len(a)] = 0
			continue
		}
		var k uint32
		for i, u := range a {
			t := uint64(u)*uint64(v) + uint64(c[i+j]) + uint64(k)
			c[i+j] = uint32(t)
			k = uint32(t >> 32)
		}
		c[j+len(a)] = k
	}
	m.traceMPNUnary(c, 427, true)
}
