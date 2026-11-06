package polynomial

// Source: src/math/polynomial/polynomial.cpp:5661,5671,5682,5688,5691.

import "io"

func (m *MultivariatePolynomialCore) emitLazardTrace(line int, body func(*polynomialTraceSink) error) error {
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace("Lazard") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("Lazard", "Se_Lazard", "src/math/polynomial/polynomial.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			formatErr = body(&polynomialTraceSink{writer: w})
			return formatErr
		})
	return formatErr
}

func (m *MultivariatePolynomialCore) traceLazardEntry(y, p Polynomial, n uint32) error {
	return m.emitLazardTrace(5661, func(sink *polynomialTraceSink) error {
		if e := m.traceDirectPolynomial(sink, "lc_S_d: ", y); e != nil {
			return e
		}
		if e := m.traceDirectPolynomial(sink, "\nS_d_1: ", p); e != nil {
			return e
		}
		_ = sink.WriteLiteral("\nn: ")
		_ = sink.WriteUnsigned(n)
		return sink.WriteLiteral("\n")
	})
}

func (m *MultivariatePolynomialCore) traceLazardPower(a uint32) error {
	return m.emitLazardTrace(5671, func(sink *polynomialTraceSink) error {
		_ = sink.WriteLiteral("a: ")
		_ = sink.WriteUnsigned(a)
		return sink.WriteLiteral("\n")
	})
}

func (m *MultivariatePolynomialCore) traceLazardLoop(a uint32, c Polynomial) error {
	return m.emitLazardTrace(5682, func(sink *polynomialTraceSink) error {
		_ = sink.WriteLiteral("loop a: ")
		_ = sink.WriteUnsigned(a)
		if e := m.gcdPRSTracePolynomial(sink, "\nC : ", c); e != nil {
			return e
		}
		return sink.WriteLiteral("\n")
	})
}

func (m *MultivariatePolynomialCore) traceLazardBranch(c Polynomial) error {
	return m.emitLazardTrace(5688, func(sink *polynomialTraceSink) error {
		if e := m.gcdPRSTracePolynomial(sink, "if, C: ", c); e != nil {
			return e
		}
		return sink.WriteLiteral("\n")
	})
}

func (m *MultivariatePolynomialCore) traceLazardFinal(c, y Polynomial) error {
	return m.emitLazardTrace(5691, func(sink *polynomialTraceSink) error {
		if e := m.gcdPRSTracePolynomial(sink, "C: ", c); e != nil {
			return e
		}
		if e := m.gcdPRSTracePolynomial(sink, "\nY: ", y); e != nil {
			return e
		}
		return sink.WriteLiteral("\n")
	})
}
