package polynomial

// Source: src/math/polynomial/polynomial.cpp:3556-3604; src/util/trace.h:40-46.

import (
	"io"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *MultivariatePolynomialCore) gcdPRSTracePolynomial(sink *polynomialTraceSink, label string, p Polynomial) error {
	_ = sink.WriteLiteral(label)
	return m.displayIntoCore(p, sink, false)
}

// traceDirectPolynomial preserves an explicit polynomial::display source call.
// polynomial_ref insertion uses gcdPRSTracePolynomial and its public wrapper.
func (m *MultivariatePolynomialCore) traceDirectPolynomial(sink *polynomialTraceSink, label string, p Polynomial) error {
	_ = sink.WriteLiteral(label)
	return m.displayPolynomialDirect(p, sink, false)
}

func (m *MultivariatePolynomialCore) gcdPRSTraceNumeral(sink *polynomialTraceSink, label string, a *unumeral.Int) error {
	_ = sink.WriteLiteral(label)
	value, err := m.modular.String(a)
	if err != nil {
		return err
	}
	return sink.WriteLiteral(value)
}

func (m *MultivariatePolynomialCore) traceGCDPRSEntry(u, v Polynomial, x uint32) error {
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace("polynomial_gcd") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("polynomial_gcd", "gcd_prs", "src/math/polynomial/polynomial.cpp", 3556,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("gcd prs with x")
			_ = sink.WriteUnsigned(x)
			_ = sink.WriteLiteral(" for\n")
			formatErr = m.traceDirectPolynomial(sink, "u: ", u)
			if formatErr != nil {
				return formatErr
			}
			_ = sink.WriteLiteral("\n")
			formatErr = m.traceDirectPolynomial(sink, "v: ", v)
			if formatErr != nil {
				return formatErr
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}

func (m *MultivariatePolynomialCore) traceGCDPRSContent(u, v Polynomial, iu, iv *unumeral.Int, cu, cv, pu, pv, dr Polynomial, da *unumeral.Int) error {
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace("polynomial_gcd_detail") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("polynomial_gcd_detail", "gcd_prs", "src/math/polynomial/polynomial.cpp", 3572,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("After GCD of the content\n")
			for _, row := range [...]struct {
				label string
				value Polynomial
			}{{"u: ", u}, {"v: ", v}} {
				formatErr = m.traceDirectPolynomial(sink, row.label, row.value)
				if formatErr != nil {
					return formatErr
				}
				_ = sink.WriteLiteral("\n")
			}
			for _, row := range [...]struct {
				label string
				value *unumeral.Int
			}{{"i_u: ", iu}, {"i_v: ", iv}} {
				formatErr = m.gcdPRSTraceNumeral(sink, row.label, row.value)
				if formatErr != nil {
					return formatErr
				}
				_ = sink.WriteLiteral("\n")
			}
			for _, row := range [...]struct {
				label string
				value Polynomial
			}{{"c_u: ", cu}, {"c_v: ", cv}, {"pp_u: ", pu}, {"pp_v: ", pv}, {"d_r: ", dr}} {
				formatErr = m.gcdPRSTracePolynomial(sink, row.label, row.value)
				if formatErr != nil {
					return formatErr
				}
				_ = sink.WriteLiteral("\n")
			}
			formatErr = m.gcdPRSTraceNumeral(sink, "d_a: ", da)
			if formatErr != nil {
				return formatErr
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}

func (m *MultivariatePolynomialCore) traceGCDPRSIteration(counter uint32, pu, pv Polynomial, delta uint32) error {
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace("polynomial_gcd_detail") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("polynomial_gcd_detail", "gcd_prs", "src/math/polynomial/polynomial.cpp", 3591,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("iteration: ")
			_ = sink.WriteUnsigned(counter)
			_ = sink.WriteLiteral("\ngcd loop\n")
			formatErr = m.gcdPRSTracePolynomial(sink, "pp_u: ", pu)
			if formatErr != nil {
				return formatErr
			}
			_ = sink.WriteLiteral("\n")
			formatErr = m.gcdPRSTracePolynomial(sink, "pp_v: ", pv)
			if formatErr != nil {
				return formatErr
			}
			_ = sink.WriteLiteral("\ndelta: ")
			_ = sink.WriteUnsigned(delta)
			return sink.WriteLiteral("\n")
		})
	return formatErr
}

func (m *MultivariatePolynomialCore) traceGCDPRSZero(pv Polynomial) error {
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace("polynomial") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("polynomial", "gcd_prs", "src/math/polynomial/polynomial.cpp", 3597,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("rem is zero...\n")
			formatErr = m.gcdPRSTracePolynomial(sink, "pp_v: ", pv)
			if formatErr != nil {
				return formatErr
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}

func (m *MultivariatePolynomialCore) traceGCDPRSConstant(rem, dr Polynomial, da *unumeral.Int) error {
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace("polynomial") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("polynomial", "gcd_prs", "src/math/polynomial/polynomial.cpp", 3604,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			formatErr = m.gcdPRSTracePolynomial(sink, "rem is a constant: ", rem)
			if formatErr != nil {
				return formatErr
			}
			_ = sink.WriteLiteral("\n")
			formatErr = m.gcdPRSTracePolynomial(sink, "r: ", dr)
			if formatErr != nil {
				return formatErr
			}
			_ = sink.WriteLiteral("\n")
			formatErr = m.gcdPRSTraceNumeral(sink, "d_a: ", da)
			if formatErr != nil {
				return formatErr
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}
