package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1376,1400; config_mpq.

import "io"

func (c *RationalContext) tracePropagateClause(clause *RationalClause) error {
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
				bodyErr = c.displayInequality(out, atom)
				if bodyErr != nil {
					return bodyErr
				}
			}
			_, _ = io.WriteString(out, "\n")
			return nil
		})
	return bodyErr
}

func (c *RationalContext) tracePropagatingInequality(atom *RationalInequality) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("propagate_clause") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("propagate_clause", "propagate_clause", "src/math/subpaving/subpaving_t_def.h", 1400,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, "propagating inequality: ")
			bodyErr = c.displayInequality(out, atom)
			if bodyErr != nil {
				return bodyErr
			}
			_, _ = io.WriteString(out, "\n")
			return nil
		})
	return bodyErr
}
