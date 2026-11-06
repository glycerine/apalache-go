package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:243-246; config_mpq.

import "io"

// The source ostream records badbit but keeps evaluating later display
// operands. Let the underlying trace stream retain its failure while bound
// display continues to perform its numeral reads and formatting.
type splitTraceStreamWriter struct{ out io.Writer }

func (w splitTraceStreamWriter) Write(p []byte) (int, error) {
	_, _ = w.out.Write(p)
	return len(p), nil
}

func (c *RationalContext) traceSplitBounds(left, right *RationalNode) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("subpaving_int_split") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("subpaving_int_split", "operator()", "src/math/subpaving/subpaving_t_def.h", 243,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			_, _ = io.WriteString(out, "LEFT:\n")
			if err := c.displayNodeBounds(out, left); err != nil {
				bodyErr = err
				return err
			}
			_, _ = io.WriteString(out, "\nRIGHT:\n")
			if err := c.displayNodeBounds(out, right); err != nil {
				bodyErr = err
				return err
			}
			return nil
		})
	return bodyErr
}
