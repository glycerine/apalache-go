package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1501,1525,1603; config_mpf.

import (
	"io"
	"strconv"
)

func (c *ArbitraryContext) traceMonomialStart(variable uint32) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("propagate_monomial") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("propagate_monomial", "propagate_monomial", "src/math/subpaving/subpaving_t_def.h", 1501,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, "propagate_monomial: ")
			bodyErr = c.writeVariable(out, variable)
			if bodyErr != nil {
				return bodyErr
			}
			_, _ = io.WriteString(out, "\n")
			return nil
		})
	return bodyErr
}

func (c *ArbitraryContext) traceMonomialFlags(foundZero, foundUnbounded bool) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("propagate_monomial") {
		return nil
	}
	_ = tags.EmitConditional("propagate_monomial", "propagate_monomial", "src/math/subpaving/subpaving_t_def.h", 1525,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			zero, unbounded := "0", "0"
			if foundZero {
				zero = "1"
			}
			if foundUnbounded {
				unbounded = "1"
			}
			_, _ = io.WriteString(out, "found_zero: "+zero+", found_unbounded: "+unbounded+"\n")
			return nil
		})
	return nil
}

func (c *ArbitraryContext) traceMonomialDownward(variable uint32, j int) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("propagate_monomial") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("propagate_monomial", "propagate_monomial_downward", "src/math/subpaving/subpaving_t_def.h", 1603,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, "propagate_monomial_downward: ")
			bodyErr = c.writeVariable(out, variable)
			if bodyErr != nil {
				return bodyErr
			}
			_, _ = io.WriteString(out, ", j: "+strconv.Itoa(j)+"\n")
			bodyErr = c.displayDefinition(out, variable, false)
			if bodyErr != nil {
				return bodyErr
			}
			_, _ = io.WriteString(out, "\n")
			return nil
		})
	return bodyErr
}
