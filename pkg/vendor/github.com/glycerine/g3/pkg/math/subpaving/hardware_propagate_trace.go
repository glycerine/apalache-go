package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1703; config_hwf.

import (
	"io"
	"strconv"
)

// The queued bound has already been consumed when the source emits this
// record, before it inspects watchers or the bound's own definition.
func (c *HardwareContext) tracePropagatingBound(b *HardwareBound) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_propagate") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_propagate", "propagate", "src/math/subpaving/subpaving_t_def.h", 1703,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, "propagate: ")
			if err := c.displayInequality(out, b.variable, &b.value, b.lower, b.open, c.displayVariable); err != nil {
				bodyErr = err
				return err
			}
			_, _ = io.WriteString(out, ", timestamp: ")
			_, _ = io.WriteString(out, strconv.FormatUint(b.timestamp, 10))
			_, _ = io.WriteString(out, "\n")
			return nil
		})
	return bodyErr
}
