package bitset

// Source: src/util/hash.h:25-96; src/util/hash.cpp:22-63.

func mix(a, b, c uint32) (uint32, uint32, uint32) {
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

func hashBytes(n, init uint32, word, character func(uint32) uint32) uint32 {
	a, b, c := uint32(0x9e3779b9), uint32(0x9e3779b9), init
	i := uint32(0)
	for n-i >= 12 {
		a += word(i)
		b += word(i + 4)
		c += word(i + 8)
		a, b, c = mix(a, b, c)
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
			c += v << ((j - 7) * 8)
		}
	}
	_, _, c = mix(a, b, c)
	return c
}

func wordBytes(words []uint32, n, init uint32) uint32 {
	if uint64(n) > uint64(len(words))*4 {
		panic("hash word byte length out of range")
	}
	return hashBytes(n, init,
		func(i uint32) uint32 { return words[i/4] },
		func(i uint32) uint32 {
			return uint32(int32(int8(byte(words[i/4] >> (8 * (i % 4))))))
		})
}

func wordReader(n, init uint32, word func(uint32) uint32) uint32 {
	return hashBytes(n, init,
		func(i uint32) uint32 { return word(i / 4) },
		func(i uint32) uint32 {
			return uint32(int32(int8(byte(word(i/4) >> (8 * (i % 4))))))
		})
}
