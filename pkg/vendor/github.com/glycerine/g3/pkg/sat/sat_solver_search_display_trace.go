package sat

// Source: src/sat/sat_solver.cpp:1184,1237; retained util/trace TRACE sites.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *Solver) traceSearchDisplay(function string, line int, prefix string) {
	tags := s.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("sat", function, "src/sat/sat_solver.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			if prefix != "" {
				_, _ = io.WriteString(w, prefix)
			}
			return s.traceUserDisplayInto(w)
		})
}

func (s *Solver) traceSearchInitialized() {
	s.traceSearchDisplay("init_search", 1184, "")
}

func (s *Solver) traceSimplifyCompleted() {
	value := "0"
	if !s.propagation.inconsistent {
		value = "1"
	}
	s.traceSearchDisplay("simplify_problem", 1237, "consistent: "+value+"\n")
}
