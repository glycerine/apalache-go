package numeral

// Source: src/util/mpn.cpp:257-259.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *NaturalManager) sourceTraceTags() *diagnostic.TraceTags {
	if m.trace != nil {
		return m.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

// Both numeric division and decimal conversion reach div_normalize.
func (m *NaturalManager) traceDivisionNormalize(numer, denom []uint32) {
	_ = m.sourceTraceTags().EmitConditional("mpn_norm", "div_normalize", "src/util/mpn.cpp", 257,
		func() bool { return true }, func(w io.Writer) error {
			_ = writeNumeralDisplayPart(w, "Normalized: n_numer=")
			_ = m.DisplayRaw(w, numer)
			_ = writeNumeralDisplayPart(w, " n_denom=")
			_ = m.DisplayRaw(w, denom)
			_ = diagnostic.WriteTraceEndl(w)
			return nil
		})
}
