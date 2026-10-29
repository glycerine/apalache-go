package sat

// Source: src/sat/sat_solver.cpp:2374; retained util/trace TRACE site.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *satSolverConflictAnalysis) traceDroppedLemmaLiteral(l Literal) {
	tags := s.propagation.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("sat", "minimize_lemma", "src/sat/sat_solver.cpp", 2374,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "drop: "+l.String()+"\n")
			return nil
		})
}
