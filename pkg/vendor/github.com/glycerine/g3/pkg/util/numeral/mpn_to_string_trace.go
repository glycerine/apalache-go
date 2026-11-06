package numeral

// Source: src/util/mpn.cpp:362,395.

import (
	"io"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *NaturalManager) traceToCStringInput(a limbView) {
	_ = m.sourceTraceTags().EmitConditional("mpn_to_string", "to_string", "src/util/mpn.cpp", 362,
		func() bool { return true }, func(w io.Writer) error {
			if err := writeNumeralDisplayPart(w, "[mpn] to_string "); err != nil {
				return err
			}
			if err := writeNumeralDisplayPart(w, "["); err != nil {
				return err
			}
			for i := a.size; i > 0; {
				i--
				v, err := readDecimalLimb(a, i)
				if err != nil {
					return err
				}
				if err := writeNumeralDisplayPart(w, strconv.FormatUint(uint64(v), 10)); err != nil {
					return err
				}
				if i != 0 {
					if err := writeNumeralDisplayPart(w, "|"); err != nil {
						return err
					}
				}
			}
			if err := writeNumeralDisplayPart(w, "]"); err != nil {
				return err
			}
			return writeNumeralDisplayPart(w, " == ")
		})
}

func (m *NaturalManager) traceToCStringResult(dst sourceDecimalDestination) {
	_ = m.sourceTraceTags().EmitConditional("mpn_to_string", "to_string", "src/util/mpn.cpp", 395,
		func() bool { return true }, func(w io.Writer) error {
			var out strings.Builder
			for i := uint64(0); ; i++ {
				c, err := dst.read(i)
				if err != nil {
					return err
				}
				if c == 0 {
					if err := writeNumeralDisplayPart(w, out.String()); err != nil {
						return err
					}
					return diagnostic.WriteTraceEndl(w)
				}
				out.WriteByte(c)
			}
		})
}
