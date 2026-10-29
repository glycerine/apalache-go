package memory

// Source: src/util/util.h:391-398.

// MegabytesToBytes converts the source unsigned-MB parameter to native size_t
// bytes. UINT_MAX is the source unlimited sentinel.
func MegabytesToBytes(mb uint32) uint64 {
	return megabytesToBytesLimit(mb, uint64(^uint(0)))
}

func megabytesToBytesLimit(mb uint32, maxSizeT uint64) uint64 {
	if mb == ^uint32(0) {
		return maxSizeT
	}
	bytes := uint64(mb) << 20
	if bytes > maxSizeT {
		return maxSizeT
	}
	return bytes
}
