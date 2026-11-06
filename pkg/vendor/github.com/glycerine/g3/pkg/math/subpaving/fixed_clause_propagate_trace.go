package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1376,1400; config_mpfx.

import "io"

func (c *FixedContext) tracePropagateClause(clause *FixedClause) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("propagate_clause") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("propagate_clause", "propagate_clause", "src/math/subpaving/subpaving_t_def.h", 1376,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, "propagate using:\n")
			for i, atom := range clause.atoms {
				if i != 0 {
					_, _ = io.WriteString(out, " or ")
				}
				if err := c.displayInequality(out, atom); err != nil {
					bodyErr = err
					return err
				}
			}
			_, _ = io.WriteString(out, "\n")
			return nil
		})
	return bodyErr
}

func (c *FixedContext) tracePropagatingInequality(a *FixedInequality) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("propagate_clause") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("propagate_clause", "propagate_clause", "src/math/subpaving/subpaving_t_def.h", 1400,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, "propagating inequality: ")
			if err := c.displayInequality(out, a); err != nil {
				bodyErr = err
				return err
			}
			_, _ = io.WriteString(out, "\n")
			return nil
		})
	return bodyErr
}
