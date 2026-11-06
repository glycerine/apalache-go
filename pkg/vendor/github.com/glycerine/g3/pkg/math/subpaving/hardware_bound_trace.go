package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:563-566; config_hwf.

import (
	"io"
	"strconv"
)

func (c *HardwareContext) traceNewBound(b *HardwareBound) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_mk_bound") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_mk_bound", "mk_bound", "src/math/subpaving/subpaving_t_def.h", 563,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, "mk_bound: ")
			bodyErr = c.displayInequality(out, b.variable, &b.value, b.lower, b.open, c.displayVariable)
			if bodyErr != nil {
				return bodyErr
			}
			_, _ = io.WriteString(out, "\ntimestamp: ")
			_, _ = io.WriteString(out, strconv.FormatUint(b.timestamp, 10))
			_, _ = io.WriteString(out, "\n")
			return nil
		})
	return bodyErr
}

func (c *HardwareContext) traceBoundConflict(n *HardwareNode) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_mk_bound") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_mk_bound", "mk_bound", "src/math/subpaving/subpaving_t_def.h", 565,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, "conflict\n")
			bodyErr = c.displayNodeBounds(out, n, c.displayVariable)
			return bodyErr
		})
	return bodyErr
}
