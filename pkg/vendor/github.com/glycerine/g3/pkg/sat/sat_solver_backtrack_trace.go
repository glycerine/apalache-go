package sat

// Source: src/sat/sat_solver.cpp:2193,2533; retained util/trace TRACE sites.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *satSolverClauseLifecycle) tracePushScope() {
	tags := s.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("sat_verbose", "push", "src/sat/sat_solver.cpp", 2533,
		func() bool { return true }, func(w io.Writer) error {
			v := s.propagation.variables
			n, err := v.trail.Size(v.trailRef)
			if err != nil {
				return err
			}
			_, _ = io.WriteString(w, "q:"+strconv.FormatUint(uint64(v.qhead), 10)+
				" trail: "+strconv.FormatUint(uint64(n), 10)+"\n")
			return nil
		})
}

func (s *satSolverConflictResolution) traceForgetPhase(l Literal) {
	tags := s.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("forget_phase", "forget_phase_of_vars", "src/sat/sat_solver.cpp", 2193,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "forgeting phase of l: "+l.String()+"\n")
			return nil
		})
}
