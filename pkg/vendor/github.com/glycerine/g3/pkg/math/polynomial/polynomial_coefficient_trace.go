package polynomial

// Source: src/math/polynomial/polynomial.cpp:3267; src/util/trace.h:40-46.

import (
	"io"
	"strconv"
)

func (m *MultivariatePolynomialCore) traceCoefficientPolynomial(p Polynomial, x, k uint32) error {
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace("coeff_bug") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("coeff_bug", "coeff", "src/math/polynomial/polynomial.cpp", 3267,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("p: ")
			formatErr = m.displayPolynomialDirect(p, sink, false)
			if formatErr != nil {
				return formatErr
			}
			_ = sink.WriteLiteral("\nx: ")
			_ = sink.WriteLiteral(strconv.FormatUint(uint64(x), 10))
			_ = sink.WriteLiteral(", k: ")
			_ = sink.WriteLiteral(strconv.FormatUint(uint64(k), 10))
			return sink.WriteLiteral("\n")
		})
	return formatErr
}
