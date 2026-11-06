package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1170-1175; config_mpq.

import (
	"io"
	"strconv"
)

// The source diagnostic intentionally prints strict > while the predicate
// below uses >=. Equal timestamps therefore print result 0 and still run.
func writeRationalMayPropagateTail(out io.Writer, bound, visited uint64) {
	flag := "0"
	if bound > visited {
		flag = "1"
	}
	_, _ = io.WriteString(out, "\nresult: "+flag+", "+strconv.FormatUint(bound, 10)+", "+strconv.FormatUint(visited, 10)+"\n")
}

func (c *RationalContext) traceMayPropagateClause(b *RationalBound, clause *RationalClause) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("may_propagate_bug") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("may_propagate_bug", "may_propagate", "src/math/subpaving/subpaving_t_def.h", 1172,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			if err := c.displayBound(out, b); err != nil {
				bodyErr = err
				return err
			}
			_, _ = io.WriteString(out, " | ")
			for i, atom := range clause.atoms {
				if i != 0 {
					_, _ = io.WriteString(out, " or ")
				}
				if err := c.displayInequality(out, atom); err != nil {
					bodyErr = err
					return err
				}
			}
			writeRationalMayPropagateTail(out, b.timestamp, clause.visited)
			return nil
		})
	return bodyErr
}

func (c *RationalContext) traceMayPropagateDefinition(b *RationalBound, variable uint32, visited uint64) error {
	tags := c.polynomialTraceTags()
	if !tags.ShouldTrace("may_propagate_bug") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("may_propagate_bug", "may_propagate", "src/math/subpaving/subpaving_t_def.h", 1172,
		func() bool { return true }, func(w io.Writer) error {
			out := splitTraceStreamWriter{out: w}
			if err := c.displayBound(out, b); err != nil {
				bodyErr = err
				return err
			}
			_, _ = io.WriteString(out, " | ")
			if err := c.displayDefinition(out, variable, false); err != nil {
				bodyErr = err
				return err
			}
			writeRationalMayPropagateTail(out, b.timestamp, visited)
			return nil
		})
	return bodyErr
}
