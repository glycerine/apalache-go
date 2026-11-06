package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1703; config_mpq.

import (
	"io"
	"strconv"
)

// The queued bound has already been consumed when the source emits this
// record, before it inspects watchers or the bound's own definition.
func (c *RationalContext) tracePropagatingBound(b *RationalBound) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_propagate") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_propagate", "propagate", "src/math/subpaving/subpaving_t_def.h", 1703,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, "propagate: ")
			if err := c.displayBound(out, b); err != nil {
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
