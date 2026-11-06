package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:243-246; config_mpff.

import "io"

func (c *FastContext) traceSplitBounds(left, right *FastNode) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_int_split") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_int_split", "operator()", "src/math/subpaving/subpaving_t_def.h", 243,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, "LEFT:\n")
			if err := c.displayNodeBounds(out, left); err != nil {
				bodyErr = err
				return err
			}
			_, _ = io.WriteString(out, "\nRIGHT:\n")
			if err := c.displayNodeBounds(out, right); err != nil {
				bodyErr = err
				return err
			}
			return nil
		})
	return bodyErr
}
