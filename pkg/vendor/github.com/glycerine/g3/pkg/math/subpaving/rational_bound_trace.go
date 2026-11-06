package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:563-566; config_mpq.

import (
	"io"
	"strconv"
)

// mk_bound emits this record after the node owns the bound and before its
// conflict check or timestamp increment.
func (c *RationalContext) traceNewBound(b *RationalBound) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_mk_bound") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_mk_bound", "mk_bound", "src/math/subpaving/subpaving_t_def.h", 563,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, "mk_bound: ")
			if err := c.displayBound(out, b); err != nil {
				bodyErr = err
				return err
			}
			_, _ = io.WriteString(out, "\ntimestamp: ")
			_, _ = io.WriteString(out, strconv.FormatUint(b.timestamp, 10))
			_, _ = io.WriteString(out, "\n")
			return nil
		})
	return bodyErr
}

// The conflict record reads the node's published lower/upper views before
// set_conflict removes it from the consistent-leaf list.
func (c *RationalContext) traceBoundConflict(n *RationalNode) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_mk_bound") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_mk_bound", "mk_bound", "src/math/subpaving/subpaving_t_def.h", 565,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, "conflict\n")
			if err := c.displayNodeBounds(out, n); err != nil {
				bodyErr = err
				return err
			}
			return nil
		})
	return bodyErr
}
