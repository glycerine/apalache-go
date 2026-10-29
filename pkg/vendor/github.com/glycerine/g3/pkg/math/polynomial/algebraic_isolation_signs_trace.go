package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:2578-2592,2598.

import (
	"errors"
	"io"
)

func (m *AlgebraicRootCore) traceIsolationSignsRoots(pm *MultivariatePolynomialManager, p Polynomial, values PolynomialAlgebraicEvaluation, roots *AlgebraicVector) error {
	tags := m.config.Rationals.Integers().BorrowTraceTags()
	if !tags.ShouldTrace("isolate_roots_bug") {
		return nil
	}
	if m.isolationSigns.traceVariablesConstructed {
		return ErrAlgebraicPending
	}
	m.isolationSigns.traceRunning = true
	previous := pm.core.algebraicTraceRunning
	pm.core.algebraicTraceRunning = true
	defer func() {
		m.isolationSigns.traceRunning = false
		pm.core.algebraicTraceRunning = previous
	}()
	return m.emitRootTraceTag("isolate_roots_bug", "isolate_roots", 2578, func(w io.Writer) error {
		return m.traceIsolationSignsRootsBody(w, pm, p, values, roots)
	})
}

func (m *AlgebraicRootCore) traceIsolationSignsRootsBody(w io.Writer, pm *MultivariatePolynomialManager, p Polynomial, values PolynomialAlgebraicEvaluation, roots *AlgebraicVector) (cause error) {
	sink := &polynomialTraceSink{writer: w}
	out := algebraicPredicateTraceStream{w}
	_ = out.AppendCString("p: ")
	m.rootTrace.polynomial = pm.core
	if e := pm.core.displayIntoCoreWithReceipt(p, sink, false, &m.rootTrace.poly); e != nil {
		return e
	}
	_ = out.AppendCString("\n")
	xs := &m.isolationSigns.traceVariables
	variableEmbedded(xs, pm.core.memory, true)
	m.isolationSigns.traceVariablesConstructed = true
	defer func() {
		cleanup := m.cleanupIsolationSignsTraceVariables(false)
		if cleanup != nil {
			m.isolationSigns.sourceStop = cleanup
		}
		cause = errors.Join(cause, cleanup)
	}()
	if e := pm.core.varsCore(p, xs); e != nil {
		return e
	}
	for i := 0; i < len(xs.values); i++ {
		x := xs.values[i]
		has, e := values.Contains(x)
		if e != nil {
			return e
		}
		if !has {
			continue
		}
		_ = out.AppendCString("x")
		_ = sink.WriteUnsigned(x)
		_ = out.AppendCString(" -> ")
		a, e := values.Algebraic(x)
		if e != nil {
			return e
		}
		location, e := m.locationCleanup(a)
		if e != nil {
			return e
		}
		if e = m.displayRootAtLocationWithReceipt(out, location, false, &m.rootTrace.root); e != nil {
			return e
		}
		_ = out.AppendCString(" ")
		// Source calls x2v(xs[i]) again for the interval insertion.
		a, e = values.Algebraic(x)
		if e != nil {
			return e
		}
		location, e = m.locationCleanup(a)
		if e != nil {
			return e
		}
		if e = m.displayIntervalAtLocationWithReceipts(algebraicComparisonTraceWriter{w}, location, &m.rootTrace.insertions[0], &m.rootTrace.insertions[1]); e != nil {
			return e
		}
		_ = out.AppendCString("\n")
	}
	for i := 0; i < len(roots.values); i++ {
		_ = out.AppendCString("root[i]: ")
		location, e := m.locationCleanup(&roots.values[i])
		if e != nil {
			return e
		}
		if e = m.displayRootAtLocationWithReceipt(out, location, false, &m.rootTrace.root); e != nil {
			return e
		}
		_ = out.AppendCString("\n")
	}
	return nil
}

func (m *AlgebraicRootCore) traceIsolationSignsSample() error {
	tags := m.config.Rationals.Integers().BorrowTraceTags()
	if !tags.ShouldTrace("isolate_roots_bug") {
		return nil
	}
	if m.isolationSigns.traceVariablesConstructed {
		return ErrAlgebraicPending
	}
	m.isolationSigns.traceRunning = true
	defer func() { m.isolationSigns.traceRunning = false }()
	return m.emitRootTraceTag("isolate_roots_bug", "isolate_roots", 2598, func(w io.Writer) error {
		out := algebraicPredicateTraceStream{w}
		_ = out.AppendCString("w: ")
		if e := m.traceIsolationRoot(out, &m.isolationSigns.sample); e != nil {
			return e
		}
		return out.AppendCString("\n")
	})
}

func (m *AlgebraicRootCore) cleanupIsolationSignsTraceVariables(owning bool) error {
	s := &m.isolationSigns
	if !s.traceVariablesConstructed {
		return nil
	}
	if !owning {
		if e := m.rootTraceSourceCause(); e != nil {
			return e
		}
	}
	if e := s.traceVariables.CloseChecked(); e != nil {
		return e
	}
	s.traceVariablesConstructed = false
	return nil
}
