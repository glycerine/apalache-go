package sat

// Source: src/sat/sat_simplifier.cpp:210,216; retained util/trace sites.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *satSimplifier) traceFinalizeCleanupBranch() {
	tags := s.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	line, message := 216, "skipping cleanup...\n"
	if s.needCleanup {
		line, message = 210, "cleanning watches...\n"
	}
	_ = tags.EmitConditional("after_simplifier", "scoped_finalize_fn", "src/sat/sat_simplifier.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			_, err := io.WriteString(w, message)
			return err
		})
}

func (s *satSimplifier) traceFinalizeDisplays() {
	if s.cleaner == nil || s.cleaner.solver == nil {
		return
	}
	tags := s.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("after_simplifier", "scoped_finalize_fn", "src/sat/sat_simplifier.cpp", 223,
		func() bool { return true }, func(w io.Writer) error {
			if err := s.cleaner.solver.traceUserDisplayInto(w); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "model_converter:\n"); err != nil {
				return err
			}
			return s.cleaner.solver.traceModelConverterInto(w)
		})
}
