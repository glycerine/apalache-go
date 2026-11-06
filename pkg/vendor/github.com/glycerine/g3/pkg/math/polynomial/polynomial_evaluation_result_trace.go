package polynomial

// Source: src/math/polynomial/polynomial.cpp:6504-6505.

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

func polynomialEvaluationResultHeader(out *polynomialTraceSink, start, end, x uint32) {
	_ = out.WriteLiteral("result for start: ")
	polynomialEvaluationTraceIndices(out, start, end, x)
	_ = out.WriteLiteral("\nr: ")
}
func (m *MultivariatePolynomialManager) traceEvaluationRationalResult(start, end, x uint32, value *unumeral.Rational) (err error) {
	s := &m.evaluation.trace
	enabled, e := m.beginEvaluationTrace(s, false)
	if e != nil || !enabled {
		return e
	}
	defer func() { err = m.finishEvaluationTrace(s, err) }()
	return m.emitEvaluationTrace(s, 6504, func(out *polynomialTraceSink) error {
		polynomialEvaluationResultHeader(out, start, end, x)
		if e := m.evaluation.rational.DisplayToTracked(out, value, &s.insertions[0]); e != nil {
			return e
		}
		return out.WriteLiteral("\n")
	})
}
func (m *MultivariatePolynomialManager) traceEvaluationIntegerResult(start, end, x uint32, value *unumeral.Int) (err error) {
	s := &m.evaluation.trace
	enabled, e := m.beginEvaluationTrace(s, false)
	if e != nil || !enabled {
		return e
	}
	defer func() { err = m.finishEvaluationTrace(s, err) }()
	return m.emitEvaluationTrace(s, 6504, func(out *polynomialTraceSink) error {
		polynomialEvaluationResultHeader(out, start, end, x)
		// mpzzp_manager::display forwards to its raw integer manager.
		if e := m.core.integer.DisplayToTracked(out, value, &s.insertions[0]); e != nil {
			return e
		}
		return out.WriteLiteral("\n")
	})
}
func (m *MultivariatePolynomialManager) traceEvaluationIntervalResult(start, end, x uint32, value *unumeral.BinaryInterval) (err error) {
	s := &m.evaluation.trace
	enabled, e := m.beginEvaluationTrace(s, false)
	if e != nil || !enabled {
		return e
	}
	defer func() { err = m.finishEvaluationTrace(s, err) }()
	return m.emitEvaluationTrace(s, 6504, func(out *polynomialTraceSink) error {
		polynomialEvaluationResultHeader(out, start, end, x)
		if e := m.evaluation.interval.DisplayTracked(out, value, &s.insertions[0], &s.insertions[1]); e != nil {
			return e
		}
		return out.WriteLiteral("\n")
	})
}
func (m *MultivariatePolynomialManager) traceEvaluationAlgebraicResult(start, end, x uint32, value *Algebraic) (err error) {
	s := &m.algebraicEvaluation.trace
	enabled, e := m.beginEvaluationTrace(s, true)
	if e != nil || !enabled {
		return e
	}
	defer func() { err = m.finishEvaluationTrace(s, err) }()
	return m.emitEvaluationTrace(s, 6504, func(out *polynomialTraceSink) error {
		polynomialEvaluationResultHeader(out, start, end, x)
		// anum_manager::display selects display_decimal with default precision10.
		if e := m.algebraicEvaluation.vm.displayDecimalWithReceipt(out, value, 10, &s.decimal); e != nil {
			return e
		}
		return out.WriteLiteral("\n")
	})
}
