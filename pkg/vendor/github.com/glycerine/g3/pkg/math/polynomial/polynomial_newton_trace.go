package polynomial

// Source: src/math/polynomial/polynomial.cpp:2833-2901; src/util/trace.h:40-46.

import (
	"io"
	"strconv"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func newtonIndexLabel(name string, index uint32) string {
	return name + "[" + strconv.FormatUint(uint64(index), 10) + "]: "
}

func (n *PolynomialNewtonInterpolator) traceAddEntry(input *unumeral.Int, output Polynomial) error {
	m := n.owner
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace("newton") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("newton", "add", "src/math/polynomial/polynomial.cpp", 2833,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			value, err := m.modular.String(input)
			if err != nil {
				formatErr = err
				return err
			}
			_ = sink.WriteLiteral(value)
			_ = sink.WriteLiteral(" -> ")
			formatErr = m.displayPolynomialDirect(output, sink, false)
			if formatErr != nil {
				return formatErr
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}

func (n *PolynomialNewtonInterpolator) traceNumeral(line int, function, label string, value *unumeral.Int) error {
	m := n.owner
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace("newton") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("newton", function, "src/math/polynomial/polynomial.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral(label)
			valueText, err := m.modular.String(value)
			if err != nil {
				formatErr = err
				return err
			}
			_ = sink.WriteLiteral(valueText)
			return sink.WriteLiteral("\n")
		})
	return formatErr
}

func (n *PolynomialNewtonInterpolator) tracePolynomial(line int, function, label string, value Polynomial) error {
	m := n.owner
	tags := m.monomials.monomialTraceTags()
	if !tags.ShouldTrace("newton") {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional("newton", function, "src/math/polynomial/polynomial.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			_ = sink.WriteLiteral(label)
			formatErr = m.displayIntoCore(value, sink, false)
			if formatErr != nil {
				return formatErr
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}
