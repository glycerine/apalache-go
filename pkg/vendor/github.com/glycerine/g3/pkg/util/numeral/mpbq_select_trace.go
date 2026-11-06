package numeral

// Source: src/util/mpbq.cpp:750-754.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// BindTrace borrows the numeric trace stream shared with this manager's
// integer operations. Nil restores the process trace tags.
func (m *BinaryRationalManager) BindTrace(tags *diagnostic.TraceTags) error {
	if err := m.open(); err != nil {
		return err
	}
	return m.Z.BindTrace(tags)
}

func (m *BinaryRationalManager) traceSelectSmallRationalLower(qm *RationalManager, lower *Rational, upper *BinaryRational) {
	tags := m.Z.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("select_small", "select_small_core", "src/util/mpbq.cpp", 752,
		func() bool { return true }, func(w io.Writer) error {
			if err := writeNumeralDisplayPart(w, "lower (q): "); err != nil {
				return err
			}
			low, err := qm.String(lower)
			if err != nil {
				return err
			}
			if err := writeNumeralDisplayPart(w, low); err != nil {
				return err
			}
			if err := writeNumeralDisplayPart(w, ", upper (bq): "); err != nil {
				return err
			}
			high, err := m.String(upper)
			if err != nil {
				return err
			}
			if err := writeNumeralDisplayPart(w, high); err != nil {
				return err
			}
			return writeNumeralDisplayPart(w, "\n")
		})
}
