package base

// Source: src/muz/base/dl_util.cpp string_to_uint64.

// StringToUint64 follows scanf's prefix conversion: leading ASCII whitespace
// and an optional sign are accepted, and trailing non-digits are ignored.
func StringToUint64(text string) (uint64, bool) {
	offset := 0
	for offset < len(text) {
		switch text[offset] {
		case ' ', '\t', '\n', '\r', '\v', '\f':
			offset++
		default:
			goto sign
		}
	}
sign:
	negative := false
	if offset < len(text) && (text[offset] == '+' || text[offset] == '-') {
		negative = text[offset] == '-'
		offset++
	}
	value, _, ok := ReadUint64(text, offset)
	if !ok {
		return 0, false
	}
	if negative {
		value = 0 - value
	}
	return value, true
}
