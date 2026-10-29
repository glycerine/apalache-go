package numeral

// Source: src/util/mpn.cpp:350-354.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
)

// Source display_raw requests m+n+1 limbs while numer owns only m+n.
// Emit its opening bracket, then stop before reading an uninitialized limb.
func (m *NaturalManager) traceDivisionManyStep(quotient, remainder uint64, n, numerSize, borrow uint32) {
	_ = m.sourceTraceTags().EmitConditional("mpn_div", "div_n", "src/util/mpn.cpp", 350,
		func() bool { return true }, func(w io.Writer) error {
			_ = writeNumeralDisplayPart(w, "q_hat=")
			_ = writeNumeralDisplayPart(w, strconv.FormatUint(quotient, 10))
			_ = writeNumeralDisplayPart(w, " r_hat=")
			_ = writeNumeralDisplayPart(w, strconv.FormatUint(remainder, 10))
			_ = writeNumeralDisplayPart(w, " ms=")
			_ = m.DisplayRaw(w, m.Division[2][:n])
			_ = writeNumeralDisplayPart(w, " new numer=")
			_ = writeNumeralDisplayPart(w, "[")
			// display_raw starts at index m+n when asked for m+n+1 limbs.
			// This is beyond numer's logical size even if its allocation has
			// spare capacity. No later TRACE operand is reached.
			return arena.ErrBounds
		})
}
