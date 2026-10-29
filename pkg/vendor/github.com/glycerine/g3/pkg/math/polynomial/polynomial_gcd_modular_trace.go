package polynomial

// Source: src/math/polynomial/polynomial.cpp:4239-4343; src/util/trace.h:40-46.

import (
	"io"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *MultivariatePolynomialManager) traceModGCDPolynomials(tag string, line int, prefix string, polys ...Polynomial) error {
	c := m.core
	tags := c.monomials.monomialTraceTags()
	if !tags.ShouldTrace(tag) {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional(tag, "mod_gcd", "src/math/polynomial/polynomial.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral(prefix)
			for i, p := range polys {
				if i != 0 {
					_ = sink.WriteLiteral("\nv: ")
				}
				formatErr = c.displayPolynomialDirect(p, sink, true)
				if formatErr != nil {
					return formatErr
				}
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}

func (m *MultivariatePolynomialManager) traceModGCDPrime(p *unumeral.Int) error {
	c := m.core
	tags := c.monomials.monomialTraceTags()
	if !tags.ShouldTrace("mgcd") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("mgcd", "mod_gcd", "src/math/polynomial/polynomial.cpp", 4288,
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

func (m *MultivariatePolynomialManager) traceModGCDLiteral(line int, body string) {
	_ = m.core.monomials.monomialTraceTags().Emit("mgcd", "mod_gcd", "src/math/polynomial/polynomial.cpp", line, body)
}
