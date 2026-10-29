package polynomial

// Source: src/math/polynomial/polynomial.cpp:3641-3642; src/util/trace.h:46.

import "io"

func (m *MultivariatePolynomialManager) traceEuclidNonUnivariate(u, v Polynomial) error {
	c := m.core
	tags := c.monomials.monomialTraceTags()
	if !tags.ShouldTrace("mgcd") {
		return nil
	}
	one, err := c.IsUnivariate(u)
	if err != nil {
		return err
	}
	if one {
		one, err = c.IsUnivariate(v)
		if err != nil {
			return err
		}
		if one {
			return nil
		}
	}
	var formatErr error
	_ = tags.EmitConditional("mgcd", "euclid_gcd", "src/math/polynomial/polynomial.cpp", 3641,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("euclid_gcd, polynomials are not univariate\n")
			formatErr = c.displayPolynomialDirect(u, sink, false)
			if formatErr != nil {
				return formatErr
			}
			_ = sink.WriteLiteral("\n")
			formatErr = c.displayPolynomialDirect(v, sink, false)
			if formatErr != nil {
				return formatErr
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}
