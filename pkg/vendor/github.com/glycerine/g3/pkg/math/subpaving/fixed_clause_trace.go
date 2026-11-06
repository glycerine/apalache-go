package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:841,865-868; config_mpfx.

import "io"

// The source emits this record after all clause references, watchers and
// context-list publication, using stable variable order.
func (c *FixedContext) traceNewClause(clause *FixedClause) error {
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

// The unit TRACE precedes the source inc_ref and list publication.
func (c *FixedContext) traceUnitInequality(a *FixedInequality) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving", "add_unit_clause", "src/math/subpaving/subpaving_t_def.h", 866,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			if err := c.displayInequality(out, a); err != nil {
				bodyErr = err
				return err
			}
			_, _ = io.WriteString(out, "\n")
			return nil
		})
	return bodyErr
}
