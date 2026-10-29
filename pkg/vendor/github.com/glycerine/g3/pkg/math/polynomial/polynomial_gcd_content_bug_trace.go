package polynomial

// Source: src/math/polynomial/polynomial.cpp:4002-4006; src/util/trace.h:46.

import (
	"errors"
	"io"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *MultivariatePolynomialManager) traceICCPZPXContentBug(g, pp Polynomial, x uint32, receipt *polynomialDividesInvocationReceipt) (err error) {
	c := m.core
	tags := c.monomials.monomialTraceTags()
	if !tags.ShouldTrace("content_bug") {
		return nil
	}
	divides, err := c.dividesWithReceipt(g, pp, receipt)
	if err != nil || divides {
		return err
	}
	var modulus unumeral.Int
	defer func() { err = errors.Join(err, c.integer.Del(&modulus)) }()
	if err = c.modular.Modulus(&modulus); err != nil {
		return err
	}
	var formatErr error
	_ = tags.EmitConditional("content_bug", "iccp_ZpX", "src/math/polynomial/polynomial.cpp", 4002,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("GF(")
			value, e := c.integer.String(&modulus)
			if e != nil {
				formatErr = e
				return e
			}
			_ = sink.WriteLiteral(value)
			_ = sink.WriteLiteral(")\npp: ")
			formatErr = c.displayPolynomialDirect(pp, sink, false)
			if formatErr != nil {
				return formatErr
			}
			_ = sink.WriteLiteral("\nvar: x")
			_ = sink.WriteUnsigned(x)
			_ = sink.WriteLiteral("\ncontent: ")
			formatErr = c.displayIntoCore(g, sink, false)
			if formatErr != nil {
				return formatErr
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}
