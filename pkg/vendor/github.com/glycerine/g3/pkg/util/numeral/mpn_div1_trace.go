package numeral

// Source: src/util/mpn.cpp:296-302.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *NaturalManager) traceDivisionOneStep(j uint32, quotient, temp, denom, ms uint64, numer limbView, borrow bool) {
	_ = m.sourceTraceTags().EmitConditional("mpn_div1", "div_1", "src/util/mpn.cpp", 296,
		func() bool { return true }, func(w io.Writer) error {
			_ = writeNumeralDisplayPart(w, "j=")
			_ = writeNumeralDisplayPart(w, strconv.FormatUint(uint64(j), 10))
			_ = writeNumeralDisplayPart(w, " q_hat=")
			_ = writeNumeralDisplayPart(w, strconv.FormatUint(quotient, 10))
			_ = writeNumeralDisplayPart(w, " r_hat=")
			_ = writeNumeralDisplayPart(w, strconv.FormatUint(temp%denom, 10))
			_ = writeNumeralDisplayPart(w, " ms=")
			_ = writeNumeralDisplayPart(w, strconv.FormatUint(ms, 10))
			_ = writeNumeralDisplayPart(w, " new numer=")
			_ = m.DisplayRaw(w, numer.external[:numer.size])
			_ = writeNumeralDisplayPart(w, " borrow=")
			bit := "0"
			if borrow {
				bit = "1"
			}
			_ = writeNumeralDisplayPart(w, bit)
			_ = diagnostic.WriteTraceEndl(w)
			return nil
		})
}
