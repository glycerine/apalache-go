package polynomial

// Source: algebraic_numbers.cpp:2288-2443 multivariate isolation TRACE sites.

import (
	"io"
	"strconv"
)

func (m *AlgebraicRootCore) traceMultivariateIsolationLiteral(line int, text string) error {
	return m.emitRootTraceTag("isolate_roots", "isolate_roots", line, func(w io.Writer) error {
		return (algebraicPredicateTraceStream{w}).AppendCString(text)
	})
}
func (m *AlgebraicRootCore) traceMultivariateIsolationPolynomial(pm *MultivariatePolynomialManager, p Polynomial, line int, prefix string) error {
	if !m.config.Rationals.Integers().BorrowTraceTags().ShouldTrace("isolate_roots") {
		return nil
	}
	previous := pm.core.algebraicTraceRunning
	pm.core.algebraicTraceRunning = true
	defer func() { pm.core.algebraicTraceRunning = previous }()
	return m.emitRootTraceTag("isolate_roots", "isolate_roots", line, func(w io.Writer) error {
		sink := &polynomialTraceSink{writer: w}
		_ = sink.WriteLiteral(prefix)
		m.rootTrace.polynomial = pm.core
		if e := pm.core.displayIntoCoreWithReceipt(p, sink, false, &m.rootTrace.poly); e != nil {
			return e
		}
		return sink.WriteLiteral("\n")
	})
}
func (m *AlgebraicRootCore) traceMultivariateIsolationResultant(pm *MultivariatePolynomialManager, i, y uint32, py, q Polynomial) error {
	if !m.config.Rationals.Integers().BorrowTraceTags().ShouldTrace("isolate_roots") {
		return nil
	}
	previous := pm.core.algebraicTraceRunning
	pm.core.algebraicTraceRunning = true
	defer func() { pm.core.algebraicTraceRunning = previous }()
	return m.emitRootTraceTag("isolate_roots", "isolate_roots", 2360, func(w io.Writer) error {
		sink := &polynomialTraceSink{writer: w}
		_ = sink.WriteLiteral("resultant loop i: " + strconv.FormatUint(uint64(i), 10) + ", y: x" + strconv.FormatUint(uint64(y), 10) + "\np_y: ")
		m.rootTrace.polynomial = pm.core
		if e := pm.core.displayIntoCoreWithReceipt(py, sink, false, &m.rootTrace.poly); e != nil {
			return e
		}
		_ = sink.WriteLiteral("\nq: ")
		if e := pm.core.displayIntoCoreWithReceipt(q, sink, false, &m.rootTrace.poly); e != nil {
			return e
		}
		return sink.WriteLiteral("\n")
	})
}

func (m *AlgebraicRootCore) traceMultivariateIsolationVariableCount(count uint32) error {
	return m.emitRootTraceTag("isolate_roots", "isolate_roots", 2332, func(w io.Writer) error {
		return (algebraicPredicateTraceStream{w}).AppendCString("there are " + strconv.FormatUint(uint64(count), 10) + " variables assigned to nonbasic numbers...\n")
	})
}

func (m *AlgebraicRootCore) traceIsolationRoot(out algebraicPredicateTraceStream, a *Algebraic) error {
	l, e := m.locationCleanup(a)
	if e != nil {
		return e
	}
	return m.displayRootAtLocationWithReceipt(out, l, false, &m.rootTrace.root)
}
func (m *AlgebraicRootCore) traceMultivariateIsolationLinearRoot(a *Algebraic) error {
	return m.emitRootTraceTag("isolate_roots", "isolate_roots", 2398, func(w io.Writer) error {
		out := algebraicPredicateTraceStream{w}
		_ = out.AppendCString("after trivial solving p has only one root:\n")
		if e := m.traceIsolationRoot(out, a); e != nil {
			return e
		}
		return out.AppendCString("\n")
	})
}
func (m *AlgebraicRootCore) traceMultivariateIsolationRecovery(pm *MultivariatePolynomialManager, q Polynomial, z uint32, a *Algebraic) error {
	if !m.config.Rationals.Integers().BorrowTraceTags().ShouldTrace("isolate_roots") {
		return nil
	}
	previous := pm.core.algebraicTraceRunning
	pm.core.algebraicTraceRunning = true
	defer func() { pm.core.algebraicTraceRunning = previous }()
	return m.emitRootTraceTag("isolate_roots", "isolate_roots", 2434, func(w io.Writer) error {
		sink := &polynomialTraceSink{writer: w}
		_ = sink.WriteLiteral("invoking isolate_roots with q2:\n")
		m.rootTrace.polynomial = pm.core
		if e := pm.core.displayIntoCoreWithReceipt(q, sink, false, &m.rootTrace.poly); e != nil {
			return e
		}
		_ = sink.WriteLiteral("\nz: x" + strconv.FormatUint(uint64(z), 10) + " -> ")
		out := algebraicPredicateTraceStream{w}
		if e := m.traceIsolationRoot(out, a); e != nil {
			return e
		}
		return out.AppendCString("\n")
	})
}
