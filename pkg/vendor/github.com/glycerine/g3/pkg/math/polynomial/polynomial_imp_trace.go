package polynomial

// Source: src/math/polynomial/polynomial.cpp:2356-2367.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *MultivariatePolynomialManager) traceLeakedPolynomials() error {
	tags := m.core.monomials.monomialTraceTags()
	if !tags.ShouldTrace("polynomial") {
		return nil
	}
	if m.destructorTraceRunning {
		return ErrPolynomialPending
	}
	m.destructorTraceRunning = true
	defer func() { m.destructorTraceRunning = false }()
	var operandErr error
	_ = tags.EmitConditional("polynomial", "~imp", "src/math/polynomial/polynomial.cpp", 2357,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("leaked polynomials\n")
			for _, p := range m.core.polynomials {
				if p == (Polynomial{}) {
					continue
				}
				operandErr = m.core.displayPolynomialDirect(p, sink, false)
				if operandErr != nil {
					return operandErr
				}
				_ = sink.WriteLiteral("\n")
			}
			return nil
		})
	return operandErr
}

func polynomialImpSourceAssert(line int32, expression string, condition bool) error {
	if !polynomialSourceDebug {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil || !enabled || condition {
		return e
	}
	if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/polynomial.cpp", line, expression); e != nil {
		return errors.Join(ErrPolynomialAssertion, e)
	}
	return nil
}
