package polynomial

// Source: src/math/polynomial/polynomial.cpp:3678-3760; src/util/trace.h:40-46.

import (
	"io"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type craTraceValue struct {
	label string
	value *unumeral.Int
}

func (m *MultivariatePolynomialManager) traceCRANumbers(line int, values ...craTraceValue) error {
	c := m.core
	tags := c.monomials.monomialTraceTags()
	if !tags.ShouldTrace("CRA") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("CRA", "CRA_combine_images", "src/math/polynomial/polynomial.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			for _, part := range values {
				_ = sink.WriteLiteral(part.label)
				value, err := c.modular.String(part.value)
				if err != nil {
					formatErr = err
					return err
				}
				_ = sink.WriteLiteral(value)
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}

func (m *MultivariatePolynomialManager) traceCRAImages(a, b Polynomial) error {
	c := m.core
	tags := c.monomials.monomialTraceTags()
	if !tags.ShouldTrace("CRA") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("CRA", "CRA_combine_images", "src/math/polynomial/polynomial.cpp", 3678,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral("C1: ")
			formatErr = c.displayPolynomialDirect(a, sink, false)
			if formatErr != nil {
				return formatErr
			}
			_ = sink.WriteLiteral("\nC2: ")
			formatErr = c.displayPolynomialDirect(b, sink, false)
			if formatErr != nil {
				return formatErr
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}

func (m *MultivariatePolynomialManager) traceCRABezout(b1, b2, inv1, inv2 *unumeral.Int) error {
	return m.traceCRANumbers(3688,
		craTraceValue{"b1: ", b1}, craTraceValue{", b2: ", b2},
		craTraceValue{", inv1: ", inv1}, craTraceValue{", inv2: ", inv2})
}
func (m *MultivariatePolynomialManager) traceCRAInverse(inv1, inv2 *unumeral.Int) error {
	return m.traceCRANumbers(3694, craTraceValue{"inv1: ", inv1}, craTraceValue{", inv2: ", inv2})
}
func (m *MultivariatePolynomialManager) traceCRAMultipliers(a1, a2 *unumeral.Int) error {
	return m.traceCRANumbers(3699, craTraceValue{"a1: ", a1}, craTraceValue{", a2: ", a2})
}
func (m *MultivariatePolynomialManager) traceCRABounds(lower, upper *unumeral.Int) error {
	return m.traceCRANumbers(3709, craTraceValue{"lower: ", lower}, craTraceValue{", upper: ", upper})
}
func (m *MultivariatePolynomialManager) traceCRABranch(line int, body string) {
	tags := m.core.monomials.monomialTraceTags()
	_ = tags.Emit("CRA", "CRA_combine_images", "src/math/polynomial/polynomial.cpp", line, body)
}
func (m *MultivariatePolynomialManager) traceCRAEqual(a Polynomial, ai uint32, b Polynomial, bi uint32, newA, tmp1, tmp2, tmp3 *unumeral.Int) error {
	tags := m.core.monomials.monomialTraceTags()
	if !tags.ShouldTrace("CRA") {
		return nil
	}
	left, err := m.core.coefficientCore(a, ai)
	if err != nil {
		return err
	}
	right, err := m.core.coefficientCore(b, bi)
	if err != nil {
		return err
	}
	return m.traceCRANumbers(3748,
		craTraceValue{"C1->a(i1): ", &left}, craTraceValue{", C2->a(i2): ", &right},
		craTraceValue{", new_a: ", newA}, craTraceValue{"\ntmp1: ", tmp1},
		craTraceValue{", tmp2: ", tmp2}, craTraceValue{", tmp3: ", tmp3})
}
