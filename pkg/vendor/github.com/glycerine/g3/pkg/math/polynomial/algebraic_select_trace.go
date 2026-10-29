package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:2523-2525.

import "io"

// Each interval finishes before the next begins, so the actual insertion
// receipts can be reused. A failed formatter remains in the root trace owner.
func (m *AlgebraicRootCore) traceSelect(prev, curr *Algebraic) error {
	return m.emitRootTraceTag("algebraic_select", "select", 2523, func(w io.Writer) error {
		out := algebraicComparisonTraceWriter{w}
		for i, value := range [2]*Algebraic{prev, curr} {
			label := "prev: "
			if i != 0 {
				label = "curr: "
			}
			_ = writeAlgebraicIntervalPart(out, label)
			location, e := m.locationCleanup(value)
			if e != nil {
				return e
			}
			if e = m.displayIntervalAtLocationWithReceipts(out, location, &m.rootTrace.insertions[0], &m.rootTrace.insertions[1]); e != nil {
				return e
			}
			_ = writeAlgebraicIntervalPart(out, "\n")
		}
		return nil
	})
}
