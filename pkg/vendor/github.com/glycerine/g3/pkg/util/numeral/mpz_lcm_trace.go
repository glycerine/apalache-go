package numeral

// Source: src/util/mpz.cpp:983-1013.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// traceLCM preserves the source TRACE point after each successful operation.
// stage zero is the GCD record, before any of the three result branches.
func (m *IntegerManager) traceLCM(a, b, c, gcd *Int, stage, line int) {
	tags := m.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("lcm_bug", "lcm", "src/util/mpz.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			writeNumber := func(v *Int) error {
				s, err := m.String(v)
				if err != nil {
					return err
				}
				return writeNumeralDisplayPart(w, s)
			}
			if stage == 0 {
				if err := writeNumeralDisplayPart(w, "gcd("); err != nil {
					return err
				}
			} else if err := writeNumeralDisplayPart(w, strconv.Itoa(stage)+". lcm("); err != nil {
				return err
			}
			if err := writeNumber(a); err != nil {
				return err
			}
			if err := writeNumeralDisplayPart(w, ", "); err != nil {
				return err
			}
			if err := writeNumber(b); err != nil {
				return err
			}
			if err := writeNumeralDisplayPart(w, ") = "); err != nil {
				return err
			}
			if stage == 0 {
				if err := writeNumber(gcd); err != nil {
					return err
				}
			} else if err := writeNumber(c); err != nil {
				return err
			}
			return writeNumeralDisplayPart(w, "\n")
		})
}
