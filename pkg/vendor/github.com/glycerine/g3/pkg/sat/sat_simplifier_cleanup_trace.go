package sat

// Source: src/sat/sat_simplifier.cpp:255; retained util/trace site.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *satSimplifier) traceCleanupClauses() {
	tags := s.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("sat", "cleanup_clauses", "src/sat/sat_simplifier.cpp", 255,
		func() bool { return true }, func(w io.Writer) error {
			_, err := io.WriteString(w, "cleanup_clauses\n")
			return err
		})
}
