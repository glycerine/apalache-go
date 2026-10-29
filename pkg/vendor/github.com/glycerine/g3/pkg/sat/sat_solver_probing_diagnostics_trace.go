package sat

// Source: src/sat/sat_solver.cpp:3061; retained util/trace TRACE site.

import (
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *satSolverClauseLifecycle) traceMissedPropagation(c StoredClauseRef) {
	tags := s.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("sat_missed_prop", "check_missed_propagation", "src/sat/sat_solver.cpp", 3061,
		func() bool { return true }, func(w io.Writer) error {
			clause, err := satTraceClause(s.propagation.clauses, c)
			if err != nil {
				return err
			}
			var b strings.Builder
			b.WriteString("missed_propagation: ")
			b.WriteString(clause)
			b.WriteByte('\n')
			n, err := s.propagation.clauses.Size(c)
			if err != nil {
				return err
			}
			for i := uint32(0); i < n; i++ {
				l, err := s.propagation.clauses.Literal(c, i)
				if err != nil {
					return err
				}
				value, err := s.propagation.variables.valueLiteral(l)
				if err != nil {
					return err
				}
				b.WriteString(l.String())
				b.WriteString(": ")
				b.WriteString(value.String())
				b.WriteByte('\n')
			}
			_, _ = io.WriteString(w, b.String())
			return nil
		})
}
