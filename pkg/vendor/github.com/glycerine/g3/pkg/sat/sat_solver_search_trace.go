package sat

// Source: src/sat/sat_solver.cpp:1027; retained util/trace TRACE site.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *Solver) traceConflictAtLevelZero() {
	tags := s.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("sat", "propagate_and_backjump_step", "src/sat/sat_solver.cpp", 1027,
		func() bool { return true }, func(w io.Writer) error {
			_, err := io.WriteString(w, "conflict at level 0\n")
			return err
		})
}
