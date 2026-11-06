package polynomial

// Source: src/math/polynomial/polynomial.cpp:3770-3860; src/util/trace.h:40-46.

import (
	"io"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *MultivariatePolynomialManager) traceUniModGCDPolynomials(line int, prefix string, polys ...Polynomial) error {
	c := m.core
	tags := c.monomials.monomialTraceTags()
	if !tags.ShouldTrace("mgcd") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("mgcd", "uni_mod_gcd", "src/math/polynomial/polynomial.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral(prefix)
			for i, p := range polys {
				if i != 0 {
					_ = sink.WriteLiteral("\nv: ")
				}
				formatErr = c.displayPolynomialDirect(p, sink, false)
				if formatErr != nil {
					return formatErr
				}
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}

func (m *MultivariatePolynomialManager) traceUniModGCDPrime(p *unumeral.Int) error {
	c := m.core
	tags := c.monomials.monomialTraceTags()
	if !tags.ShouldTrace("mgcd") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("mgcd", "uni_mod_gcd", "src/math/polynomial/polynomial.cpp", 3805,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("trying prime: ")
			value, err := c.modular.String(p)
			if err != nil {
				formatErr = err
				return err
			}
			_ = sink.WriteLiteral(value)
			return sink.WriteLiteral("\n")
		})
	return formatErr
}

func (m *MultivariatePolynomialManager) traceUniModGCDLiteral(line int, body string) {
	_ = m.core.monomials.monomialTraceTags().Emit("mgcd", "uni_mod_gcd", "src/math/polynomial/polynomial.cpp", line, body)
}
