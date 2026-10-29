package polynomial

// Source: src/math/polynomial/polynomial.cpp:4355-4361,4390-4392.

import "io"

func (m *MultivariatePolynomialManager) emitGCDEntryTrace(tag string, line int, body func(*polynomialTraceSink) error) error {
	c := m.core
	tags := c.monomials.monomialTraceTags()
	if !tags.ShouldTrace(tag) {
		return nil
	}
	if c.gcdTraceRunning {
		return ErrPolynomialPending
	}
	c.gcdTraceRunning = true
	defer func() { c.gcdTraceRunning = false }()
	var computation error
	_ = tags.EmitConditional(tag, "gcd", "src/math/polynomial/polynomial.cpp", line, func() bool { return true }, func(w io.Writer) error {
		computation = body(&polynomialTraceSink{writer: w})
		return computation
	})
	return computation
}
func (m *MultivariatePolynomialManager) traceGCDOperand(sink *polynomialTraceSink, label string, p Polynomial, s *polynomialGCDFunctionScope) error {
	_ = sink.WriteLiteral(label)
	if m.core.displayReceiptPending(&s.traceDisplay) {
		return ErrPolynomialPending
	}
	s.traceDisplay = polynomialDisplayInvocationReceipt{}
	return m.core.displayPolynomialDirectWithReceipt(p, sink, false, &s.traceDisplay)
}
func polynomialTraceBool(sink *polynomialTraceSink, b bool) {
	if b {
		_ = sink.WriteUnsigned(1)
	} else {
		_ = sink.WriteUnsigned(0)
	}
}
func (m *MultivariatePolynomialManager) traceGCDEntry(u, v Polynomial, s *polynomialGCDFunctionScope) error {
	if e := m.emitGCDEntryTrace("gcd_calls", 4355, func(sink *polynomialTraceSink) error {
		if e := m.traceGCDOperand(sink, "gcd\nu: ", u, s); e != nil {
			return e
		}
		if e := m.traceGCDOperand(sink, "\nv: ", v, s); e != nil {
			return e
		}
		return sink.WriteLiteral("\n")
	}); e != nil {
		return e
	}
	return m.emitGCDEntryTrace("polynomial_gcd", 4356, func(sink *polynomialTraceSink) error {
		if e := m.traceGCDOperand(sink, "gcd\nu: ", u, s); e != nil {
			return e
		}
		if e := m.traceGCDOperand(sink, "\nv: ", v, s); e != nil {
			return e
		}
		for _, row := range [...]struct {
			p                Polynomial
			prefix, constant string
		}{{u, "\nis_zero(u): ", ", is_const(u): "}, {v, "is_zero(v): ", ", is_const(v): "}} {
			_ = sink.WriteLiteral(row.prefix)
			size, e := m.core.readWord(row.p, 8)
			if e != nil {
				return e
			}
			polynomialTraceBool(sink, size == 0)
			_ = sink.WriteLiteral(row.constant)
			yes, e := m.core.isConstantCore(row.p)
			if e != nil {
				return e
			}
			polynomialTraceBool(sink, yes)
			_ = sink.WriteLiteral("\n")
		}
		_ = sink.WriteLiteral("modular: ")
		yes, e := m.core.modular.Modular()
		if e != nil {
			return e
		}
		polynomialTraceBool(sink, yes)
		return sink.WriteLiteral("\n")
	})
}
func (m *MultivariatePolynomialManager) traceGCDVariables(s *polynomialGCDFunctionScope) error {
	return m.emitGCDEntryTrace("polynomial_gcd", 4390, func(sink *polynomialTraceSink) error {
		for j, label := range [...]string{"u var info\n", "v var info\n"} {
			_ = sink.WriteLiteral(label)
			for i := 0; i < len(s.powers[j]); i++ {
				pair := &s.powers[j][i]
				_ = sink.WriteVariable(pair.Var)
				if pair.Degree != 1 {
					_ = sink.WriteLiteral("^")
					_ = sink.WriteUnsigned(pair.Degree)
				}
				_ = sink.WriteLiteral(" ")
			}
			_ = sink.WriteLiteral("\n")
		}
		return nil
	})
}
