package base

// Source: src/muz/base/dl_util.cpp read_uint64.

import "math"

// ReadUint64 reads a decimal token beginning exactly at offset. next is the
// first unconsumed byte. On overflow the offending digit is not consumed,
// matching the source pointer contract.
func ReadUint64(text string, offset int) (value uint64, next int, ok bool) {
	if offset < 0 || offset >= len(text) || text[offset] < '0' || text[offset] > '9' {
		return 0, offset, false
	}
	value = uint64(text[offset] - '0')
	next = offset + 1
	for next < len(text) && text[next] >= '0' && text[next] <= '9' {
		digit := uint64(text[next] - '0')
		if value > (math.MaxUint64-digit)/10 {
			return value, next, false
		}
		value = value*10 + digit
		next++
	}
	return value, next, true
}
