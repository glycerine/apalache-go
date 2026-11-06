package bitutil

// Source: src/util/util.h:92-101.

// NextPowerOfTwo32 returns the next power of two greater than or equal to
// value using the source's uint32 wraparound arithmetic. Zero returns zero.
func NextPowerOfTwo32(value uint32) uint32 {
	value--
	value |= value >> 1
	value |= value >> 2
	value |= value >> 4
	value |= value >> 8
	value |= value >> 16
	return value + 1
}
