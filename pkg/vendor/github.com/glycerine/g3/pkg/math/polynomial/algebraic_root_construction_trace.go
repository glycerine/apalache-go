package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:655-659,668,678.

import (
	"io"
	"strconv"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type algebraicRootTraceScope struct {
	root       algebraicRootDisplayInvocationReceipt
	insertions [2]unumeral.NumeralInsertionReceipt
	running    bool
	borrow     upolyFactorTraceBorrow
	polynomial *MultivariatePolynomialCore
	poly       polynomialDisplayInvocationReceipt
	univariate upolyDisplayInvocationReceipt
}

func (m *AlgebraicRootCore) rootTracePending() bool {
	s := &m.rootTrace
	if m.rootDisplayInvocationPending(&s.root) {
		return true
	}
	for i := range s.insertions {
		if s.insertions[i].Active() {
			return true
		}
	}
	return s.polynomial != nil && s.polynomial.displayReceiptPending(&s.poly) || m.config.Polynomials != nil && m.config.Polynomials.displayReceiptPending(&s.univariate)
}
func (m *AlgebraicRootCore) rootTraceSourceCause() error {
	s := &m.rootTrace
	if m.rootDisplayInvocationPending(&s.root) {
		if e := m.rootDisplay.sourceStop; e != nil {
			return e
		}
	}
	for i := range s.insertions {
		if s.insertions[i].Active() {
			if e := s.insertions[i].SourceCause(); e != nil {
				return e
			}
		}
	}
	if s.polynomial != nil {
		if e := s.polynomial.displayReceiptSourceCause(&s.poly); e != nil {
			return e
		}
	}
	return m.config.Polynomials.displayReceiptSourceCause(&s.univariate)
}
func (m *AlgebraicRootCore) cleanupRootTrace() error {
	s := &m.rootTrace
	if s.running {
		return ErrAlgebraicPending
	}
	if e := m.cleanupRootDisplayInvocation(&s.root); e != nil {
		return e
	}
	for i := len(s.insertions) - 1; i >= 0; i-- {
		if !s.insertions[i].Active() {
			continue
		}
		if e := m.config.Rationals.CleanupSourceDisplay(&s.insertions[i]); e != nil {
			return e
		}
	}
	if s.polynomial != nil {
		if e := s.polynomial.cleanupDisplayReceipt(&s.poly); e != nil {
			return e
		}
	}
	if e := m.config.Polynomials.cleanupDisplayReceipt(&s.univariate); e != nil {
		return e
	}
	s.polynomial = nil
	s.root = algebraicRootDisplayInvocationReceipt{}
	s.insertions = [2]unumeral.NumeralInsertionReceipt{}
	s.poly = polynomialDisplayInvocationReceipt{}
	s.univariate = upolyDisplayInvocationReceipt{}
	return nil
}

func (m *AlgebraicRootCore) emitRootTrace(function string, line int, body func(io.Writer) error) error {
	return m.emitRootTraceTag("algebraic", function, line, body)
}
func (m *AlgebraicRootCore) emitRootTraceTag(tag, function string, line int, body func(io.Writer) error) error {
	tags := m.config.Rationals.Integers().BorrowTraceTags()
	if !tags.ShouldTrace(tag) {
		return nil
	}
	s := &m.rootTrace
	if s.running || m.rootTracePending() {
		return ErrAlgebraicPending
	}
	s.root = algebraicRootDisplayInvocationReceipt{}
	s.insertions = [2]unumeral.NumeralInsertionReceipt{}
	s.polynomial = nil
	s.poly = polynomialDisplayInvocationReceipt{}
	s.univariate = upolyDisplayInvocationReceipt{}
	if e := s.borrow.bindManagers([3]*UPolynomialManager{m.config.Polynomials}, tags); e != nil {
		return e
	}
	s.running = true
	defer func() { s.running = false; s.borrow.unbind() }()
	var computation error
	_ = tags.EmitConditional(tag, function, "src/math/polynomial/algebraic_numbers.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			computation = body(w)
			return computation
		})
	return computation
}

func (m *AlgebraicRootCore) traceRootPolynomial(pm *MultivariatePolynomialManager, p Polynomial) error {
	return m.tracePolynomialInput(pm, p, true)
}
func (m *AlgebraicRootCore) tracePolynomialInput(pm *MultivariatePolynomialManager, p Polynomial, root bool) error {
	if !m.config.Rationals.Integers().BorrowTraceTags().ShouldTrace("algebraic") {
		return nil
	}
	previous := pm.core.algebraicTraceRunning
	pm.core.algebraicTraceRunning = true
	defer func() { pm.core.algebraicTraceRunning = previous }()
	line, function := 637, "isolate_roots"
	if root {
		line, function = 668, "mk_root"
	}
	return m.emitRootTrace(function, line, func(w io.Writer) error {
		sink := &polynomialTraceSink{writer: w}
		_ = sink.WriteLiteral("isolating roots of: ")
		m.rootTrace.polynomial = pm.core
		if e := pm.core.displayIntoCoreWithReceipt(p, sink, false, &m.rootTrace.poly); e != nil {
			return e
		}
		return sink.WriteLiteral("\n")
	})
}

func (m *AlgebraicRootCore) traceRootExpression(p UPolynomialView, index uint32) error {
	return m.emitRootTrace("mk_root", 678, func(w io.Writer) error {
		out := upolyRemainderTraceStream{w}
		_ = out.AppendCString("mk_root " + strconv.FormatUint(uint64(index), 10) + "\n")
		if e := m.config.Polynomials.displayIntoWithReceipt(p, out, "x", false, &m.rootTrace.univariate); e != nil {
			return e
		}
		return out.AppendCString("\n")
	})
}

func (m *AlgebraicRootCore) traceRootSelection(roots *AlgebraicVector, count uint32) error {
	return m.emitRootTrace("mk_root", 655, func(w io.Writer) error {
		out := algebraicComparisonTraceWriter{w}
		_ = writeAlgebraicIntervalPart(out, "num-roots: "+strconv.FormatUint(uint64(count), 10)+"\n")
		for i := uint32(0); i < count; i++ {
			location, e := m.locationCleanup(&roots.values[i])
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
