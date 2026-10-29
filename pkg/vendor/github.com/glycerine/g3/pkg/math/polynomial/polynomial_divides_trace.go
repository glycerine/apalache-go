package polynomial

// Source: src/math/polynomial/polynomial.cpp:5155-5156,5181-5182.

import "io"

// Diagnostic locals precede the zero shortcut and the algorithm's locals.
// Their actual accepted display has a separate, generation-bound receipt.
type polynomialDividesTraceScope struct {
	invocation uint64
	running    bool
	current    polynomialDividesTraceInvocation
	display    polynomialDisplayInvocationReceipt
}
type polynomialDividesTraceInvocation struct {
	core       *MultivariatePolynomialCore
	frame      *polynomialDividesTraceScope
	invocation uint64
}

func (m *MultivariatePolynomialCore) matchesDividesTrace(r *polynomialDividesTraceInvocation) bool {
	return m != nil && m.self == m && r != nil && r.core == m && r.frame == &m.dividesTrace && r.invocation != 0 && r.invocation == m.dividesTrace.invocation
}
func (m *MultivariatePolynomialCore) dividesTraceInvocationPending(r *polynomialDividesTraceInvocation) bool {
	return m.matchesDividesTrace(r) && m.displayReceiptPending(&m.dividesTrace.display)
}
func (m *MultivariatePolynomialCore) dividesTraceSourceCause(r *polynomialDividesTraceInvocation) error {
	if !m.matchesDividesTrace(r) {
		return nil
	}
	return m.displayReceiptSourceCause(&m.dividesTrace.display)
}
func (m *MultivariatePolynomialCore) cleanupDividesTrace(r *polynomialDividesTraceInvocation) error {
	if !m.matchesDividesTrace(r) {
		return nil
	}
	if m.dividesTrace.running {
		return ErrPolynomialPending
	}
	return m.cleanupDisplayReceipt(&m.dividesTrace.display)
}
func (m *MultivariatePolynomialCore) acceptDividesTrace(r *polynomialDividesTraceInvocation) error {
	s := &m.dividesTrace
	if s.running || m.displayReceiptPending(&s.display) {
		return ErrPolynomialPending
	}
	if m.matchesDividesTrace(r) {
		return nil
	}
	if s.invocation == ^uint64(0) {
		return ErrPolynomialPending
	}
	*s = polynomialDividesTraceScope{invocation: s.invocation + 1}
	s.current = polynomialDividesTraceInvocation{m, s, s.invocation}
	*r = s.current
	return nil
}
func (m *MultivariatePolynomialCore) emitDividesTrace(tag string, line int, r *polynomialDividesTraceInvocation, body func(*polynomialTraceSink) error) error {
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace(tag) {
		return nil
	}
	if e := m.acceptDividesTrace(r); e != nil {
		return e
	}
	s := &m.dividesTrace
	s.running = true
	defer func() { s.running = false }()
	var computation error
	_ = tags.EmitConditional(tag, "divides", "src/math/polynomial/polynomial.cpp", line, func() bool { return true }, func(w io.Writer) error {
		computation = body(&polynomialTraceSink{writer: w})
		return computation
	})
	return computation
}
func (m *MultivariatePolynomialCore) traceDividesEntry(q, p Polynomial, r *polynomialDividesTraceInvocation) error {
	for _, record := range [...]struct {
		tag  string
		line int
	}{{"polynomial", 5155}, {"divides", 5156}} {
		if e := m.emitDividesTrace(record.tag, record.line, r, func(sink *polynomialTraceSink) error {
			_ = sink.WriteLiteral("divides\nq: ")
			s := &m.dividesTrace
			s.display = polynomialDisplayInvocationReceipt{}
			if e := m.displayPolynomialDirectWithReceipt(q, sink, false, &s.display); e != nil {
				return e
			}
			_ = sink.WriteLiteral("\np: ")
			s.display = polynomialDisplayInvocationReceipt{}
			if e := m.displayPolynomialDirectWithReceipt(p, sink, false, &s.display); e != nil {
				return e
			}
			return sink.WriteLiteral("\n")
		}); e != nil {
			return e
		}
	}
	return nil
}
func (m *MultivariatePolynomialCore) traceDividesMonomials(mr, mq, quotient Monomial, r *polynomialDividesTraceInvocation) error {
	return m.emitDividesTrace("polynomial", 5181, r, func(sink *polynomialTraceSink) error {
		_ = sink.WriteLiteral("m_r: ")
		if e := m.monomials.DisplayInto(mr, sink, false); e != nil {
			return e
		}
		_ = sink.WriteLiteral("\nm_q: ")
		if e := m.monomials.DisplayInto(mq, sink, false); e != nil {
			return e
		}
		_ = sink.WriteLiteral("\n")
		if quotient != (Monomial{}) {
			_ = sink.WriteLiteral("m_r_q: ")
			if e := m.monomials.DisplayInto(quotient, sink, false); e != nil {
				return e
			}
			_ = sink.WriteLiteral("\n")
		}
		return nil
	})
}
