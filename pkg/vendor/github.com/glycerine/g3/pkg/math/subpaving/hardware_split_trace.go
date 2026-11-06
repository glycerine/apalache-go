package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:243-246; config_hwf.

import "io"

func (c *HardwareContext) traceSplitBounds(left, right *HardwareNode) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_int_split") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_int_split", "operator()", "src/math/subpaving/subpaving_t_def.h", 243,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			display := c.displayVariable
			_, _ = io.WriteString(out, "LEFT:\n")
			if err := c.displayNodeBounds(out, left, display); err != nil {
				bodyErr = err
				return err
			}
			_, _ = io.WriteString(out, "\nRIGHT:\n")
			if err := c.displayNodeBounds(out, right, display); err != nil {
				bodyErr = err
				return err
			}
			return nil
		})
	return bodyErr
}
