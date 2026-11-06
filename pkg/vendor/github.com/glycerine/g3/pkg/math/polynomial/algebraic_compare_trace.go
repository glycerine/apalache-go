package polynomial

// Source: algebraic_numbers.cpp:1738-1743,1861,2708-2719.

import (
	"io"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// This inner record follows the interval and equal-polynomial exits. Its
// operands are reached only when the source trace is enabled.
func (m *AlgebraicRootCore) traceComparisonRoots(a, b *algebraicRecord) error {
	return m.emitRootTrace("compare_core", 1738, func(w io.Writer) error {
		out := algebraicTranslationTraceStream{algebraicPredicateTraceStream{w}}
		_ = out.AppendCString("comparing\n")
		for i, r := range [2]*algebraicRecord{a, b} {
			name := "a"
			if i != 0 {
				name = "b"
			}
			_ = out.AppendCString(name + ": ")
			p, e := m.sourcePolynomial(r)
			if e != nil {
				return e
			}
			if e = m.config.Polynomials.displayIntoWithReceipt(p, out, "x", false, &m.rootTrace.univariate); e != nil {
				return e
			}
			_ = out.AppendCString("\n(")
			for j, bound := range [2]*unumeral.BinaryRational{r.interval.BorrowLower(), r.interval.BorrowUpper()} {
				if j != 0 {
					_ = out.AppendCString(", ")
				}
				if e = m.config.Binary.DisplaySourceTracked(out, bound, &m.rootTrace.insertions[j]); e != nil {
					return e
				}
			}
			_ = out.AppendCString(")\ncell_" + name + "->m_minimal: ")
			_ = out.AppendUint32(r.flags & 1)
			_ = out.AppendCString("\n")
		}
		return nil
	})
}

// Suppress insertion failures only. The diagnostic stream keeps its failbit,
// while interval/numeral computations continue at the source operand positions.
type algebraicComparisonTraceWriter struct{ io.Writer }

func (w algebraicComparisonTraceWriter) Write(p []byte) (int, error) {
	_, _ = w.Writer.Write(p)
	return len(p), nil
}

func (m *AlgebraicRootCore) traceComparison(a, b *Algebraic) error {
	if m == nil || m.self != m {
		return nil
	}
	tags := m.config.Rationals.Integers().BorrowTraceTags()
	if !tags.ShouldTrace("algebraic") {
		return nil
	}
	if e := m.check(); e != nil {
		return e
	}
	m.comparisonTraceRunning = true
	defer func() { m.comparisonTraceRunning = false }()
	return m.emitRootTrace("compare", 1861, func(w io.Writer) error {
		out := algebraicTranslationTraceStream{algebraicPredicateTraceStream{w}}
		_ = out.AppendCString("comparing: ")
		if e := m.traceComparisonInterval(out, a); e != nil {
			return e
		}
		_ = out.AppendCString(" ")
		if e := m.traceComparisonInterval(out, b); e != nil {
			return e
		}
		return out.AppendCString("\n")
	})
}

func (m *AlgebraicRootCore) traceComparisonInterval(out algebraicTranslationTraceStream, a *Algebraic) error {
	location, e := m.locationCleanup(a)
	if e != nil {
		return e
	}
	q := unumeral.NewRational()
	if location != nil {
		r, e := m.load(location)
		if e != nil {
			return e
		}
		if location.kind == 1 {
			_ = out.AppendCString("(")
			for i, bound := range [2]*unumeral.BinaryRational{r.interval.BorrowLower(), r.interval.BorrowUpper()} {
				if i != 0 {
					_ = out.AppendCString(", ")
				}
				if e = m.config.Binary.DisplaySourceTracked(out, bound, &m.rootTrace.insertions[i]); e != nil {
					return e
				}
			}
			return out.AppendCString(")")
		}
		q = r.value
	}
	_ = out.AppendCString("[")
	for i := 0; i < 2; i++ {
		if i != 0 {
			_ = out.AppendCString(", ")
		}
		if e = m.config.Rationals.DisplayIntoTracked(out, &q, &m.rootTrace.insertions[i]); e != nil {
			return e
		}
	}
	return out.AppendCString("]")
}
