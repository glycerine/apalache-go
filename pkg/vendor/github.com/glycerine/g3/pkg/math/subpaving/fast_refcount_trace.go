package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:786-799; config_mpff.

import (
	"fmt"
	"io"
)

func (c *FastContext) traceInequalityIncRef(a *FastInequality) {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_ref_count") {
		return
	}
	_ = tags.EmitConditional("subpaving_ref_count", "inc_ref", "src/math/subpaving/subpaving_t_def.h", 786,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(splitTraceStreamWriter{out: w}, fmt.Sprintf("inc-ref: %p %d\n", a, a.refs))
			return nil
		})
}

func (c *FastContext) traceInequalityDecRef(a *FastInequality) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_ref_count") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_ref_count", "dec_ref", "src/math/subpaving/subpaving_t_def.h", 794,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, fmt.Sprintf("dec-ref: %p %d\n", a, a.refs))
			bodyErr = c.displayInequality(out, a)
			if bodyErr != nil {
				return bodyErr
			}
			_, _ = io.WriteString(out, "\n")
			return nil
		})
	return bodyErr
}
