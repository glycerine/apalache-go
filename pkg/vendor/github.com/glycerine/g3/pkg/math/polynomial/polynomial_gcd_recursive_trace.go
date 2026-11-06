package polynomial

// Source: src/math/polynomial/polynomial.cpp:4078-4223; src/util/trace.h:40-46.

import (
	"io"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *MultivariatePolynomialManager) traceModGCDRecPolynomials(line int, prefix string, sourceDisplay bool, polys ...Polynomial) error {
	c := m.core
	tags := c.monomials.monomialTraceTags()
	if !tags.ShouldTrace("mgcd") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("mgcd", "mod_gcd_rec", "src/math/polynomial/polynomial.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral(prefix)
			for i, p := range polys {
				if i != 0 {
					_ = sink.WriteLiteral("\nv: ")
				}
				formatErr = c.displayPolynomialDirect(p, sink, sourceDisplay)
				if formatErr != nil {
					return formatErr
				}
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}

func (m *MultivariatePolynomialManager) traceModGCDRecPoint(x uint32, value *unumeral.Int) error {
	c := m.core
	tags := c.monomials.monomialTraceTags()
	if !tags.ShouldTrace("mgcd") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("mgcd", "mod_gcd_rec", "src/math/polynomial/polynomial.cpp", 4141,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteVariable(x)
			_ = sink.WriteLiteral(" -> ")
			valueText, err := c.modular.String(value)
			if err != nil {
				formatErr = err
				return err
			}
			_ = sink.WriteLiteral(valueText)
			return sink.WriteLiteral("\n")
		})
	return formatErr
}

func (m *MultivariatePolynomialManager) traceModGCDRecResult(idx uint32, result Polynomial) error {
	c := m.core
	tags := c.monomials.monomialTraceTags()
	if !tags.ShouldTrace("mgcd") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("mgcd", "mod_gcd_rec", "src/math/polynomial/polynomial.cpp", 4223,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("idx: ")
			_ = sink.WriteUnsigned(idx)
			_ = sink.WriteLiteral("\nresult: ")
			formatErr = c.displayIntoCore(result, sink, false)
			if formatErr != nil {
				return formatErr
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}

func (m *MultivariatePolynomialManager) traceModGCDRecSkipped() {
	_ = m.core.monomials.monomialTraceTags().Emit("mgcd", "mod_gcd_rec", "src/math/polynomial/polynomial.cpp", 4177, "skipping q...\n")
}
