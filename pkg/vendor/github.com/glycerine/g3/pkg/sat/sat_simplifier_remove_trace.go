package sat

// Source: src/sat/sat_simplifier.cpp:103; retained util/trace site.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *satSimplifier) traceRemovedClause(c StoredClauseRef) {
	tags := s.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("resolution_bug", "remove_clause_core", "src/sat/sat_simplifier.cpp", 103,
		func() bool { return true }, func(w io.Writer) error {
			clause, err := satTraceClause(s.lifecycle.propagation.clauses, c)
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, "del_clause: "+clause+"\n")
			return err
		})
}
