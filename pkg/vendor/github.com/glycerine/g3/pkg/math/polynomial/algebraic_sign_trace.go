package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:1981-2182 (active branch).

import (
	"io"
	"strconv"
)

// The input polynomial manager is borrowed through each source record, even
// when only a numerical local is being printed after the evaluator returned.
func (m *AlgebraicRootCore) emitSignTrace(line int, body func(io.Writer) error) error {
	if pm := m.signEvaluationManager; pm != nil {
		previous := pm.core.algebraicTraceRunning
		pm.core.algebraicTraceRunning = true
		defer func() { pm.core.algebraicTraceRunning = previous }()
	}
	return m.emitRootTraceTag("anum_eval_sign", "eval_sign_at", line, body)
}

func (m *AlgebraicRootCore) traceSignPolynomial(pm *MultivariatePolynomialManager, p Polynomial, line int, prefix string) error {
	if !m.config.Rationals.Integers().BorrowTraceTags().ShouldTrace("anum_eval_sign") {
		return nil
	}
	previous := pm.core.algebraicTraceRunning
	pm.core.algebraicTraceRunning = true
	defer func() { pm.core.algebraicTraceRunning = previous }()
	return m.emitSignTrace(line, func(w io.Writer) error {
		sink := &polynomialTraceSink{writer: w}
		_ = sink.WriteLiteral(prefix)
		m.rootTrace.polynomial = pm.core
		if e := pm.core.displayIntoCoreWithReceipt(p, sink, false, &m.rootTrace.poly); e != nil {
			return e
		}
		return sink.WriteLiteral("\n")
	})
}
func (m *AlgebraicRootCore) traceSignRational() error {
	return m.emitSignTrace(1989, func(w io.Writer) error {
		out := algebraicComparisonTraceWriter{w}
		_ = writeAlgebraicIntervalPart(out, "all variables are assigned to rationals, value of p: ")
		if e := m.config.Rationals.DisplayToTracked(out, &m.rationalSign.value, &m.rootTrace.insertions[0]); e != nil {
			return e
		}
		return writeAlgebraicIntervalPart(out, "\n")
	})
}
func (m *AlgebraicRootCore) traceSignInterval(exact bool) error {
	line := 2024
	if exact {
		line = 2163
	}
	return m.emitSignTrace(line, func(w io.Writer) error {
		out := algebraicComparisonTraceWriter{w}
		_ = writeAlgebraicIntervalPart(out, "evaluating using intervals: ")
		if e := m.config.Intervals.DisplayTracked(out, &m.signInterval.interval, &m.rootTrace.insertions[0], &m.rootTrace.insertions[1]); e != nil {
			return e
		}
		_ = writeAlgebraicIntervalPart(out, "\n")
		if exact {
			_ = writeAlgebraicIntervalPart(out, "zero lower bound is: ")
			if e := m.config.Binary.DisplaySourceTracked(algebraicTranslationTraceStream{algebraicPredicateTraceStream{out}}, &m.signResultant.limits[1], &m.rootTrace.insertions[0]); e != nil {
				return e
			}
			_ = writeAlgebraicIntervalPart(out, "\n")
		}
		return nil
	})
}
func (m *AlgebraicRootCore) traceSignLiteral(line int, text string) error {
	return m.emitSignTrace(line, func(w io.Writer) error { return (algebraicPredicateTraceStream{w}).AppendCString(text) })
}
func (m *AlgebraicRootCore) traceSignResultant(k uint32) error {
	if !m.config.Rationals.Integers().BorrowTraceTags().ShouldTrace("anum_eval_sign") {
		return nil
	}
	pm := m.multivariate
	previous := pm.core.algebraicTraceRunning
	pm.core.algebraicTraceRunning = true
	defer func() { pm.core.algebraicTraceRunning = previous }()
	return m.emitSignTrace(2135, func(w io.Writer) error {
		sink := &polynomialTraceSink{writer: w}
		_ = sink.WriteLiteral("R: ")
		m.rootTrace.polynomial = pm.core
		if e := pm.core.displayIntoCoreWithReceipt(m.signResultant.refs[2].root.value, sink, false, &m.rootTrace.poly); e != nil {
			return e
		}
		_ = sink.WriteLiteral("\nk: " + strconv.FormatUint(uint64(k), 10) + "\nri: ")
		out := algebraicComparisonTraceWriter{w}
		if e := m.config.Intervals.DisplayTracked(out, &m.signInterval.interval, &m.rootTrace.insertions[0], &m.rootTrace.insertions[1]); e != nil {
			return e
		}
		return writeAlgebraicIntervalPart(out, "\n")
	})
}
