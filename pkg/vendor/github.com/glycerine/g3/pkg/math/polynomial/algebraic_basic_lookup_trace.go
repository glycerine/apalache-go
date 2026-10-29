package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:1953-1958.

import (
	"io"
	"strconv"
	"strings"
)

func (m *AlgebraicRootCore) traceBasicRationalLookup(x uint32, value *algebraicLocation) error {
	return m.emitRootTraceTag("var2basic", "operator()", 1957, func(w io.Writer) error {
		out := algebraicComparisonTraceWriter{w}
		_ = writeAlgebraicIntervalPart(out, "getting value of x"+strconv.FormatUint(uint64(x), 10)+" -> ")
		// qm().to_string completes the project formatter before string insertion.
		// Go's buffer supplies standard-library string storage.
		var text strings.Builder
		if e := m.displayBasicIntervalValue(algebraicComparisonTraceWriter{&text}, value, &m.rootTrace.insertions[0]); e != nil {
			return e
		}
		_ = writeAlgebraicIntervalPart(out, text.String())
		return writeAlgebraicIntervalPart(out, "\n")
	})
}
