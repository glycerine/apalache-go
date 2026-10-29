package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:1279,1320.

import (
	"io"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *AlgebraicRootCore) traceBasicRootRational(input *unumeral.Rational) error {
	return m.emitRootTraceTag("root_core", "root_core", 1279, func(w io.Writer) error {
		out := algebraicPredicateTraceStream{w}
		_ = out.AppendCString("r_a_val: ")
		if e := m.config.Rationals.DisplayIntoTracked(out, &m.basicRootScope.q, &m.rootTrace.insertions[0]); e != nil {
			return e
		}
		_ = out.AppendCString(" a_val: ")
		if e := m.config.Rationals.DisplayIntoTracked(out, input, &m.rootTrace.insertions[1]); e != nil {
			return e
		}
		return out.AppendCString("\n")
	})
}
func (m *AlgebraicRootCore) traceBasicRootPolynomial(p UPolynomialView) error {
	return m.emitRootTrace("root_core", 1320, func(w io.Writer) error {
		out := upolyRemainderTraceStream{w}
		_ = out.AppendCString("root_core:\n")
		if e := m.config.Polynomials.displayIntoWithReceipt(p, out, "x", false, &m.rootTrace.univariate); e != nil {
			return e
		}
		return out.AppendCString("\n")
	})
}
