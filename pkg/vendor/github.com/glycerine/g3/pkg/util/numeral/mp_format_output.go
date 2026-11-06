package numeral

// Source: src/util/mpz.cpp:1385-1410; src/util/mpn.cpp:360-399.

import "github.com/glycerine/g3/pkg/arena"

func characterByte(bytes []byte, i uint64) (byte, error) {
	if i >= uint64(len(bytes)) {
		return 0, arena.ErrBounds
	}
	return bytes[i], nil
}
