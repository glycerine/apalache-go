package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:1394-1429,1498-1539.

import (
	"io"
	"strconv"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type algebraicTranslationTraceStream struct{ algebraicPredicateTraceStream }

func (s algebraicTranslationTraceStream) AppendUint32(v uint32) error {
	return s.AppendCString(strconv.FormatUint(uint64(v), 10))
}

func (m *AlgebraicRootCore) traceRationalArithmeticInput(r *algebraicRecord, q *unumeral.Rational, multiply bool) error {
	function, line, label := "add", 1394, "adding"
	if multiply {
		function, line, label = "mul", 1498, "mult"
	}
	return m.emitRootTrace(function, line, func(w io.Writer) error {
		out := algebraicTranslationTraceStream{algebraicPredicateTraceStream{w}}
		_ = out.AppendCString(label + " algebraic and basic cells:\na: ")
		p, e := m.sourcePolynomial(r)
		if e != nil {
			return e
		}
		if e = m.config.Polynomials.displayIntoWithReceipt(p, out, "x", false, &m.rootTrace.univariate); e != nil {
			return e
		}
		_ = out.AppendCString(" (")
		if e = m.config.Binary.DisplaySourceTracked(out, r.interval.BorrowLower(), &m.rootTrace.insertions[0]); e != nil {
			return e
		}
		_ = out.AppendCString(", ")
		if e = m.config.Binary.DisplaySourceTracked(out, r.interval.BorrowUpper(), &m.rootTrace.insertions[0]); e != nil {
			return e
		}
		_ = out.AppendCString(")\nb: ")
		if e = m.config.Rationals.DisplayIntoTracked(out, q, &m.rootTrace.insertions[0]); e != nil {
			return e
		}
		return out.AppendCString("\n")
	})
}

func (m *AlgebraicRootCore) traceRationalArithmeticBounds(multiply bool) error {
	function, line := "add", 1417
	if multiply {
		function, line = "mul", 1525
	}
	return m.emitRootTrace(function, line, func(w io.Writer) error {
		out := algebraicTranslationTraceStream{algebraicPredicateTraceStream{w}}
		for i, prefix := range []string{"nbv: ", "\nil: ", ", iu: "} {
			_ = out.AppendCString(prefix)
			if e := m.config.Rationals.DisplayIntoTracked(out, &m.arithmetic.q[i], &m.rootTrace.insertions[0]); e != nil {
				return e
			}
		}
		return out.AppendCString("\n")
	})
}

func (m *AlgebraicRootCore) traceRationalArithmeticResult(p UPolynomialView, multiply bool) error {
	function, line := "add", 1425
	if multiply {
		function, line = "mul", 1535
	}
	return m.emitRootTrace(function, line, func(w io.Writer) error {
		out := algebraicTranslationTraceStream{algebraicPredicateTraceStream{w}}
		up := m.config.Polynomials
		if e := up.displayIntoWithReceipt(p, out, "x", false, &m.rootTrace.univariate); e != nil {
			return e
		}
		for i, prefix := range []string{", l: ", ", u: "} {
			_ = out.AppendCString(prefix)
			if e := m.config.Binary.DisplaySourceTracked(out, &m.arithmetic.b[i], &m.rootTrace.insertions[0]); e != nil {
				return e
			}
		}
		_ = out.AppendCString("\n")
		for i, prefix := range []string{"l_sign: ", "u_sign: "} {
			_ = out.AppendCString(prefix)
			sign, e := up.evalSignBinaryWithNegScopeAndReceipt(p, &m.arithmetic.b[i], nil, &m.arithmetic.evaluations[i])
			if e != nil {
				return e
			}
			_ = out.AppendInt32(int32(sign))
			_ = out.AppendCString("\n")
		}
		return nil
	})
}
