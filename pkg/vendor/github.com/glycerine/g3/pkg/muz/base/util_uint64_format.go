package base

// Source: src/muz/base/dl_util.cpp to_string(uint64_t).

import "strconv"

func Uint64Text(value uint64) string {
	return strconv.FormatUint(value, 10)
}
