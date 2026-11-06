package polynomial

// Source: src/math/polynomial/polynomial.cpp:5752,5772.

import "io"

func (m *MultivariatePolynomialCore) emitPSCTrace(line int, body func(*polynomialTraceSink) error) error {
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace("psc_chain_classic") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("psc_chain_classic", "psc_chain_optimized_core", "src/math/polynomial/polynomial.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			formatErr = body(&polynomialTraceSink{writer: w})
			return formatErr
		})
	return formatErr
}

func (m *MultivariatePolynomialCore) tracePSCEntry(p, q Polynomial) error {
	return m.emitPSCTrace(5752, func(sink *polynomialTraceSink) error {
		if e := m.traceDirectPolynomial(sink, "P: ", p); e != nil {
			return e
		}
		if e := m.traceDirectPolynomial(sink, "\nQ: ", q); e != nil {
			return e
		}
		return sink.WriteLiteral("\n")
	})
}

func (m *MultivariatePolynomialCore) tracePSCIteration(a, b, s Polynomial, d, e uint32) error {
	return m.emitPSCTrace(5772, func(sink *polynomialTraceSink) error {
		if err := m.gcdPRSTracePolynomial(sink, "A: ", a); err != nil {
			return err
		}
		if err := m.gcdPRSTracePolynomial(sink, "\nB: ", b); err != nil {
			return err
		}
		if err := m.gcdPRSTracePolynomial(sink, "\ns: ", s); err != nil {
			return err
		}
		_ = sink.WriteLiteral("\nd: ")
		_ = sink.WriteUnsigned(d)
		_ = sink.WriteLiteral(", e: ")
		_ = sink.WriteUnsigned(e)
		return sink.WriteLiteral("\n")
	})
}
