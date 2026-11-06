package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:1630,1641,1644.

import (
	"io"
	"strings"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *AlgebraicRootCore) traceInverseValue(a *Algebraic, after bool) error {
	line, label := 1630, "before inv: "
	if after {
		line, label = 1644, "after inv: "
	}
	return m.emitRootTraceTag("algebraic_bug", "inv", line, func(w io.Writer) error {
		out := algebraicTranslationTraceStream{algebraicPredicateTraceStream{w}}
		_ = out.AppendCString(label)
		location, e := m.locationCleanup(a)
		if e != nil {
			return e
		}
		e = m.displayRootAtLocationWithReceipt(out, location, false, &m.rootTrace.root)
		m.inversion.traceDisplay = m.rootTrace.root
		m.inversion.traceDisplayPending = m.rootDisplayInvocationPending(&m.inversion.traceDisplay)
		if e != nil {
			return e
		}
		_ = out.AppendCString("\n")
		// display_root may publish its cached index. Read the actual cell again.
		location, e = m.locationCleanup(a)
		if e != nil {
			return e
		}
		if location == nil || location.kind != 1 {
			return ErrAlgebraicOwner
		}
		r, e := m.load(location)
		if e != nil {
			return e
		}
		_ = out.AppendCString("(")
		for i, bound := range [2]*unumeral.BinaryRational{r.interval.BorrowLower(), r.interval.BorrowUpper()} {
			if i != 0 {
				_ = out.AppendCString(", ")
			}
			if e = m.config.Binary.DisplaySourceTracked(out, bound, &m.rootTrace.insertions[i]); e != nil {
				return e
			}
		}
		return out.AppendCString(")\n")
	})
}

func (m *AlgebraicRootCore) traceInverseBounds() error {
	return m.emitRootTraceTag("algebraic_bug", "inv", 1641, func(w io.Writer) error {
		out := algebraicPredicateTraceStream{w}
		_ = out.AppendCString("inv new_bounds: ")
		for i := range m.inversion.values {
			if i != 0 {
				_ = out.AppendCString(", ")
			}
			// qm().to_string finishes its project formatting children before the
			// string insertion. Native Go storage replaces the local string stream.
			var text strings.Builder
			if e := m.config.Rationals.DisplayIntoTracked(algebraicPredicateTraceStream{&text}, &m.inversion.values[i], &m.rootTrace.insertions[i]); e != nil {
				return e
			}
			_ = out.AppendCString(text.String())
		}
		return out.AppendCString("\n")
	})
}
