package sat

// Source: src/sat/sat_solver.cpp:2834; retained util/trace TRACE site.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *Solver) traceCollectedBinClause(l, l2 Literal) {
	tags := s.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("cleanup_bug", "collect_bin_clauses", "src/sat/sat_solver.cpp", 2834,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "collected: "+l.String()+" "+l2.String()+"\n")
			return nil
		})
}
