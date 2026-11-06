package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:841; config_mpq.

import "io"

// The clause has already acquired every atom, watcher and context-list entry
// when the source emits this record. Its stable variable order is displayed.
func (c *RationalContext) traceNewClause(clause *RationalClause) error {
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
