package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:865-868; config_mpq.

import "io"

// The source prints the inequality before inc_ref and unit publication.
// A failed trace ostream keeps its sticky state while display still evaluates
// the numeral and borrowed variable printer.
func (c *RationalContext) traceUnitInequality(a *RationalInequality) error {
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
