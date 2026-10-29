package polynomial

// Source: src/math/polynomial/polynomial.cpp:2148-2156,5971-5974,6029.

import "io"

func (m *MultivariatePolynomialManager) emitSqrtTrace(line int, body func(*polynomialTraceSink) error) error {
	tags := m.core.monomials.monomialTraceTags()
	if !tags.ShouldTrace("sqrt_bug") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("sqrt_bug", "sqrt", "src/math/polynomial/polynomial.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			formatErr = body(&polynomialTraceSink{writer: w})
			return formatErr
		})
	return formatErr
}

func (m *MultivariatePolynomialManager) traceSqrtEntry(p Polynomial) error {
	return m.emitSqrtTrace(5971, func(sink *polynomialTraceSink) error {
		c := m.core
		if e := c.traceDirectPolynomial(sink, "sqrt:    ", p); e != nil {
			return e
		}
		_ = sink.WriteLiteral("\nmin pos: ")
		min, e := c.gradedLexMinPosition(p)
		if e != nil {
			return e
		}
		_ = sink.WriteUnsigned(min)
		_ = sink.WriteLiteral("\nmax pos: ")
		max, e := c.gradedLexMaxPosition(p)
		if e != nil {
			return e
		}
		_ = sink.WriteUnsigned(max)
		return sink.WriteLiteral("\n")
	})
}

func (b *polynomialSOM) displayTraceInto(sink *polynomialTraceSink) error {
	m := b.owner
	if e := polynomialSOMPredicateSourceAssert(2149, "m_tmp_ms.size() == m_tmp_as.size()", func() (bool, error) {
		ms, e := b.ms.size(m)
		if e != nil {
			return false, e
		}
		as, e := b.as.size(m)
		return ms == as, e
	}); e != nil {
		return e
	}
	for i := uint32(0); ; i++ {
		size, e := b.as.size(m)
		if e != nil {
			return e
		}
		if i >= size {
			break
		}
		if i > 0 {
			_ = sink.WriteLiteral(" + ")
		}
		a, e := b.as.coefficient(m, i)
		if e != nil {
			return e
		}
		value, e := m.modular.String(&a)
		if e != nil {
			return e
		}
		_ = sink.WriteLiteral(value)
		_ = sink.WriteLiteral("*")
		term, e := b.ms.monomial(m, i)
		if e != nil {
			return e
		}
		if e = m.monomials.DisplayInto(term, sink, false); e != nil {
			return e
		}
	}
	return sink.WriteLiteral("\n")
}

func (m *MultivariatePolynomialManager) traceSqrtBuffers() error {
	return m.emitSqrtTrace(6029, func(sink *polynomialTraceSink) error {
		_ = sink.WriteLiteral("R: ")
		if e := m.core.som.displayTraceInto(sink); e != nil {
			return e
		}
		_ = sink.WriteLiteral("C: ")
		return m.core.som2.displayTraceInto(sink)
	})
}
