package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:841,865-868; config_hwf.

import "io"

func (c *HardwareContext) traceNewClause(clause *HardwareClause) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_clause") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_clause", "add_clause_core", "src/math/subpaving/subpaving_t_def.h", 841,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, "new clause:\n")
			for i, atom := range clause.atoms {
				if i != 0 {
					_, _ = io.WriteString(out, " or ")
				}
				bodyErr = c.displayInequality(out, atom.variable, &atom.value, atom.lower, atom.open, c.displayVariable)
				if bodyErr != nil {
					return bodyErr
				}
			}
			_, _ = io.WriteString(out, "\n")
			return nil
		})
	return bodyErr
}

func (c *HardwareContext) traceUnitInequality(atom *HardwareInequality) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving", "add_unit_clause", "src/math/subpaving/subpaving_t_def.h", 866,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			bodyErr = c.displayInequality(out, atom.variable, &atom.value, atom.lower, atom.open, c.displayVariable)
			if bodyErr != nil {
				return bodyErr
			}
			_, _ = io.WriteString(out, "\n")
			return nil
		})
	return bodyErr
}
