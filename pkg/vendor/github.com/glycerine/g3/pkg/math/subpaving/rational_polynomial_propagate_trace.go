package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1441-1484; config_mpq.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (c *RationalContext) tracePolynomialVariable(tag string, variable uint32, found bool, prefix string, line int) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace(tag) {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional(tag, "propagate_polynomial", "src/math/subpaving/subpaving_t_def.h", line,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, prefix)
			if found {
				bodyErr = c.writeVariable(out, variable)
				if bodyErr != nil {
					return bodyErr
				}
			} else {
				_, _ = io.WriteString(out, "[null]")
			}
			_, _ = io.WriteString(out, "\n")
			return nil
		})
	return bodyErr
}

func (c *RationalContext) tracePolynomialBounds(n *RationalNode) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("propagate_polynomial_detail") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("propagate_polynomial_detail", "propagate_polynomial", "src/math/subpaving/subpaving_t_def.h", 1468,
		func() bool { return true }, func(w io.Writer) error {
			bodyErr = c.displayNodeBounds(splitTraceStreamWriter{out: w}, n)
			return bodyErr
		})
	return bodyErr
}

func (c *RationalContext) tracePolynomialCoefficient(value *numeral.Rational) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("propagate_polynomial_bug") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("propagate_polynomial_bug", "propagate_polynomial", "src/math/subpaving/subpaving_t_def.h", 1441,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, "a: ")
			bodyErr = c.numbers.DisplayTo(out, value)
			_, _ = io.WriteString(out, "\n")
			return bodyErr
		})
	return bodyErr
}

func (c *RationalContext) tracePolynomialInterval(value *RationalInterval, line int, prefix string) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("propagate_polynomial_bug") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("propagate_polynomial_bug", "propagate_polynomial", "src/math/subpaving/subpaving_t_def.h", line,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, prefix)
			bodyErr = value.Display(out)
			_, _ = io.WriteString(out, "\n")
			return bodyErr
		})
	return bodyErr
}
