package hash

// Translation of src/util/hash.h and hash.cpp

import (
	"encoding/binary"
)

func Uint(a uint32) uint32 {
	a = (a + 0x7ed55d16) + (a << 12)
	a = (a ^ 0xc761c23c) ^ (a >> 19)
	a = (a + 0x165667b1) + (a << 5)
	a = (a + 0xd3a2646c) ^ (a << 9)
	a = (a + 0xfd7046c5) + (a << 3)
	return (a ^ 0xb55a4f09) ^ (a >> 16)
}
func Uint64(a uint64) uint32 {
	a = (^a) + (a << 18)
	a ^= a >> 31
	a += (a << 2) + (a << 4)
	a ^= a >> 11
	a += a << 6
	a ^= a >> 22
	return uint32(a)
}
func Combine(h1, h2 uint32) uint32 {
	h2 -= h1
	h2 ^= h1 << 8
	h1 -= h2
	h2 ^= h1 << 16
	h2 -= h1
	h2 ^= h1 << 10
	return h2
}
func UintPair(a, b uint32) uint32 { return Combine(Uint(a), Uint(b)) }

// MixState preserves all three words after one source mixing round.
func MixState(a, b, c uint32) (uint32, uint32, uint32) {
	a -= b
	a -= c
	a ^= c >> 13
	b -= c
	b -= a
	b ^= a << 8
	c -= a
	c -= b
	c ^= b >> 13
	a -= b
	a -= c
	a ^= c >> 12
	b -= c
	b -= a
	b ^= a << 16
	c -= a
	c -= b
	c ^= b >> 5
	a -= b
	a -= c
	a ^= c >> 3
	b -= c
	b -= a
	b ^= a << 10
	c -= a
	c -= b
	c ^= b >> 15
	return a, b, c
}
func Mix(a, b, c uint32) uint32 { _, _, c = MixState(a, b, c); return c }

// String preserves source baseline little-endian word loads and signed-char tails.
// Embedded NULs participate in the hash.
func String(s string, init uint32) uint32 {
	if uint64(len(s)) > uint64(^uint32(0)) {
		panic("hash string length exceeds source uint32 range")
	}
	return hashBytes(uint32(len(s)), init,
		func(i uint32) uint32 { return binary.LittleEndian.Uint32([]byte(s[i : i+4])) },
		func(i uint32) uint32 { return uint32(int32(int8(s[i]))) })
}

// WordBytes hashes the first n little-endian bytes of pointer-free word storage,
// preserving string_hash signed-char behavior without a byte-buffer allocation.
// n must not exceed four times the number of words.
func WordBytes(words []uint32, n, init uint32) uint32 {
	if uint64(n) > uint64(len(words))*4 {
		panic("hash word byte length out of range")
	}
	return hashBytes(n, init, func(i uint32) uint32 { return words[i/4] },
		func(i uint32) uint32 { return uint32(int32(int8(byte(words[i/4] >> (8 * (i % 4)))))) })
}
func hashBytes(n, init uint32, word, character func(uint32) uint32) uint32 {
	a, b, c := uint32(0x9e3779b9), uint32(0x9e3779b9), init
	i := uint32(0)
	for n-i >= 12 {
		a += word(i)
		b += word(i + 4)
		c += word(i + 8)
		a, b, c = MixState(a, b, c)
		i += 12
	}
	c += n
	for j := uint32(0); j < n-i; j++ {
		v := character(i + j)
		switch {
		case j < 4:
			a += v << (j * 8)
		case j < 8:
			b += v << ((j - 4) * 8)
		default:
			c += v << ((j - 8 + 1) * 8)
		}
	}
	return Mix(a, b, c)
}

// Composite preserves get_composite_hash's child callback visit order.
func Composite(kind, n uint32, child func(uint32) uint32) uint32 {
	if n == 0 {
		panic("composite hash requires at least one child")
	}
	a, b, c := uint32(0x9e3779b9), uint32(0x9e3779b9), uint32(11)
	switch n {
	case 1:
		a += kind
		b = child(0)
		return Mix(a, b, c)
	case 2:
		a += kind
		b += child(0)
		c += child(1)
		return Mix(a, b, c)
	case 3:
		a += child(0)
		b += child(1)
		c += child(2)
		a, b, c = MixState(a, b, c)
		a += kind
		return Mix(a, b, c)
	}
	for n >= 3 {
		n--
		a += child(n)
		n--
		b += child(n)
		n--
		c += child(n)
		a, b, c = MixState(a, b, c)
	}
	a += kind
	if n == 2 {
		b += child(1)
	}
	if n >= 1 {
		c += child(0)
	}
	return Mix(a, b, c)
}

// WordReader hashes n little-endian bytes from a word accessor, without exposing
// backing storage. The reader accepts word indices and must cover ceil(n/4)
// words. Source signed-char tail semantics are preserved.
func WordReader(n, init uint32, word func(uint32) uint32) uint32 {
	return hashBytes(n, init, func(i uint32) uint32 { return word(i / 4) }, func(i uint32) uint32 { return uint32(int32(int8(byte(word(i/4) >> (8 * (i % 4)))))) })
}
