package numeral

// Source: src/util/mpn.cpp:215, after the debug quotient/remainder check.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *NaturalManager) traceDivisionVerification(ok bool, sum limbView) {
	_ = m.sourceTraceTags().EmitConditional("mpn_dbg", "div", "src/util/mpn.cpp", 215,
		func() bool { return !ok }, func(w io.Writer) error {
			_ = writeNumeralDisplayPart(w, "DIV BUG: quot * denom + rem = ")
			_ = m.DisplayRaw(w, sum.external[:sum.size])
			_ = diagnostic.WriteTraceEndl(w)
			return nil
		})
}
