package sat

// Source: src/sat/sat_solver.cpp:1191-1202; retained util/trace TRACE site.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *Solver) traceSimplifyProblem() {
	tags := s.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("sat", "simplify_problem", "src/sat/sat_solver.cpp", 1199,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "simplify\n")
			return nil
		})
}
