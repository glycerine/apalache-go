package sat

// Source: src/sat/sat_simplifier.cpp:813,832; retained util/trace sites.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *satSimplifier) traceSubsumeSolverDisplay(line int) {
	// Partial standalone simplifier fixtures have no source solver display
	// owner. The retained full solver binds this borrowed pointer at construction.
	if s.cleaner == nil || s.cleaner.solver == nil {
		return
	}
	tags := s.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("subsumption_bug", "subsume", "src/sat/sat_simplifier.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			return s.cleaner.solver.traceUserDisplayInto(w)
		})
}

func (s *satSimplifier) traceRunSolverDisplay(tag, function string, line int) {
	if s.cleaner == nil || s.cleaner.solver == nil {
		return
	}
	tags := s.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional(tag, function, "src/sat/sat_simplifier.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			return s.cleaner.solver.traceUserDisplayInto(w)
		})
}
