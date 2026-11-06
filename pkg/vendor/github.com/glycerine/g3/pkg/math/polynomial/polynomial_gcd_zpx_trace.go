package polynomial

// Source: src/math/polynomial/polynomial.cpp:3878-4042; src/util/trace.h:40-46.

import "io"

func (m *MultivariatePolynomialManager) traceICCPZPXEntry(p Polynomial, x uint32) error {
	c := m.core
	tags := c.monomials.monomialTraceTags()
	if !tags.ShouldTrace("mgcd_detail") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("mgcd_detail", "iccp_ZpX", "src/math/polynomial/polynomial.cpp", 3878,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("iccp_ZpX, p: ")
			formatErr = c.displayPolynomialDirect(p, sink, false)
			if formatErr != nil {
				return formatErr
			}
			_ = sink.WriteLiteral("\nvar x")
			_ = sink.WriteUnsigned(x)
			return sink.WriteLiteral("\n")
		})
	return formatErr
}

func (m *MultivariatePolynomialManager) traceICCPZPXLiteral(line int, body string) {
	_ = m.core.monomials.monomialTraceTags().Emit("mgcd_detail", "iccp_ZpX", "src/math/polynomial/polynomial.cpp", line, body)
}

func (m *MultivariatePolynomialManager) traceICCPZPXFactor(x, degree uint32) {
	tags := m.core.monomials.monomialTraceTags()
	if !tags.ShouldTrace("mgcd_detail") {
		return
	}
	_ = tags.EmitConditional("mgcd_detail", "iccp_ZpX", "src/math/polynomial/polynomial.cpp", 3922,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("iccp_ZpX, all monomials contain x")
			_ = sink.WriteUnsigned(x)
			_ = sink.WriteLiteral(", dividing by x")
			_ = sink.WriteUnsigned(x)
			_ = sink.WriteLiteral("^")
			_ = sink.WriteUnsigned(degree)
			return sink.WriteLiteral("\n")
		})
}

func (m *MultivariatePolynomialManager) traceLCGlexZPXMax(monomial Monomial) error {
	c := m.core
	tags := c.monomials.monomialTraceTags()
	if !tags.ShouldTrace("mgcd_detail") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("mgcd_detail", "lc_glex_ZpX", "src/math/polynomial/polynomial.cpp", 4042,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("maximal monomial: ")
			formatErr = c.monomials.DisplayInto(monomial, sink, false)
			if formatErr != nil {
				return formatErr
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}
