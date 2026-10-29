package sat

// Source: src/sat/sat_solver.cpp:196,199,250; retained util/trace TRACE sites.

import (
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func satTraceSpanLiterals(n uint32, span satSolverLiteralSpan) (string, error) {
	var b strings.Builder
	for i := uint32(0); i < n; i++ {
		l, err := span.literal(i)
		if err != nil {
			return "", err
		}
		if i != 0 {
			b.WriteByte(' ')
		}
		b.WriteString(l.String())
	}
	return b.String(), nil
}

func (s *satSolverClauseLifecycle) traceClauseRecord(tag, function string, line int, body func(io.Writer) error) {
	tags := s.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional(tag, function, "src/sat/sat_solver.cpp", line,
		func() bool { return true }, body)
}

func (s *satSolverClauseLifecycle) traceMkClause(n uint32, span satSolverLiteralSpan, learned bool) {
	s.traceClauseRecord("sat", "mk_clause_core", 196, func(w io.Writer) error {
		lits, err := satTraceSpanLiterals(n, span)
		if err != nil {
			return err
		}
		suffix := " aux\n"
		if learned {
			suffix = " learned\n"
		}
		_, _ = io.WriteString(w, "mk_clause: "+lits+suffix)
		return nil
	})
}

func (s *satSolverClauseLifecycle) traceSimplifiedClause(n uint32, span satSolverLiteralSpan, keep bool) {
	s.traceClauseRecord("sat_mk_clause", "mk_clause_core", 199, func(w io.Writer) error {
		lits, err := satTraceSpanLiterals(n, span)
		if err != nil {
			return err
		}
		flag := "0"
		if keep {
			flag = "1"
		}
		_, _ = io.WriteString(w, "mk_clause (after simp), keep: "+flag+"\n"+lits+"\n")
		return nil
	})
}

func (s *satSolverClauseLifecycle) traceReinitClause(r StoredClauseRef) {
	s.traceClauseRecord("sat_reinit", "push_reinit_stack", 250, func(w io.Writer) error {
		clause, err := satTraceClause(s.propagation.clauses, r)
		if err != nil {
			return err
		}
		_, _ = io.WriteString(w, "adding to reinit stack: "+clause+"\n")
		return nil
	})
}
