package numeral

// Source: src/util/mpn.cpp:400-404; src/util/mpn.h:91.

import (
	"io"
	"strconv"
)

// DisplayRaw writes source-order limbs from the most significant index down.
func (*NaturalManager) DisplayRaw(out io.Writer, limbs []uint32) error {
	if e := writeNumeralDisplayPart(out, "["); e != nil {
		return e
	}
	for i := len(limbs); i > 0; {
		i--
		if e := writeNumeralDisplayPart(out, strconv.FormatUint(uint64(limbs[i]), 10)); e != nil {
			return e
		}
		if i != 0 {
			if e := writeNumeralDisplayPart(out, "|"); e != nil {
				return e
			}
		}
	}
	return writeNumeralDisplayPart(out, "]")
}
