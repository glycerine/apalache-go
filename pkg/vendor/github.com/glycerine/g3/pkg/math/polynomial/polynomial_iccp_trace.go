package polynomial

// Source: src/math/polynomial/polynomial.cpp:3454; src/util/trace.h:40-46.

import (
	"io"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *MultivariatePolynomialCore) traceICCPEntry(p Polynomial, x uint32) error {
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace("polynomial") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("polynomial", "iccp", "src/math/polynomial/polynomial.cpp", 3454,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("iccp x")
			_ = sink.WriteUnsigned(x)
			_ = sink.WriteLiteral("\n")
			formatErr = m.displayPolynomialDirect(p, sink, false)
			if formatErr != nil {
				return formatErr
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}

func (m *MultivariatePolynomialCore) traceICCPDegree(term Monomial, x, k uint32) error {
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace("polynomial") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("polynomial", "iccp", "src/math/polynomial/polynomial.cpp", 3493,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("degree of x")
			_ = sink.WriteUnsigned(x)
			_ = sink.WriteLiteral(" at ")
			formatErr = m.monomials.DisplayInto(term, sink, false)
			if formatErr != nil {
				return formatErr
			}
			_ = sink.WriteLiteral(" is ")
			_ = sink.WriteUnsigned(k)
			return sink.WriteLiteral("\n")
		})
	return formatErr
}

func (m *MultivariatePolynomialCore) traceICCPContent(p Polynomial, i *unumeral.Int, pp Polynomial) error {
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace("polynomial") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("polynomial", "iccp", "src/math/polynomial/polynomial.cpp", 3513,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("p: ")
			formatErr = m.displayPolynomialDirect(p, sink, false)
			if formatErr != nil {
				return formatErr
			}
			_ = sink.WriteLiteral("\ni: ")
			value, err := m.modular.String(i)
			if err != nil {
				formatErr = err
				return err
			}
			_ = sink.WriteLiteral(value)
			_ = sink.WriteLiteral("\npp: ")
			formatErr = m.displayIntoCore(pp, sink, false)
			if formatErr != nil {
				return formatErr
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}

func (m *MultivariatePolynomialCore) traceICCPDivisor(pp, c Polynomial) error {
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace("polynomial") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("polynomial", "iccp", "src/math/polynomial/polynomial.cpp", 3528,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("pp: ")
			formatErr = m.displayIntoCore(pp, sink, false)
			if formatErr != nil {
				return formatErr
			}
			_ = sink.WriteLiteral("\nc: ")
			formatErr = m.displayIntoCore(c, sink, false)
			if formatErr != nil {
				return formatErr
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}
