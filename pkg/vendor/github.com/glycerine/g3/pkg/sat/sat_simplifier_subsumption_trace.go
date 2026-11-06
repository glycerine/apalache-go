package sat

// Source: src/sat/sat_simplifier.cpp:435,441,444,542; retained util/trace sites.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *satSimplifier) traceBackwardSubsumption(tag string, line int, body func(io.Writer) error) {
	tags := s.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional(tag, "back_subsumption1", "src/sat/sat_simplifier.cpp", line,
		func() bool { return true }, body)
}

func (s *satSimplifier) traceSubsumed(c1, c2 StoredClauseRef) {
	s.traceBackwardSubsumption("subsumption", 435, func(w io.Writer) error {
		a1, err := s.kernel.clauseReader(c1)
		if err != nil {
			return err
		}
		left, err := satTraceClause(a1, c1)
		if err != nil {
			return err
		}
		right, err := satTraceClause(s.lifecycle.propagation.clauses, c2)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, left+" subsumed "+right+"\n")
		return err
	})
}

func (s *satSimplifier) traceSubsumed0(c1, c2 StoredClauseRef) {
	tags := s.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("subsumption", "back_subsumption0", "src/sat/sat_simplifier.cpp", 542,
		func() bool { return true }, func(w io.Writer) error {
			a1, err := s.kernel.clauseReader(c1)
			if err != nil {
				return err
			}
			left, err := satTraceClause(a1, c1)
			if err != nil {
				return err
			}
			right, err := satTraceClause(s.lifecycle.propagation.clauses, c2)
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, left+" subsumed "+right+"\n")
			return err
		})
}

func (s *satSimplifier) traceSubsumptionResolution(c1, c2 StoredClauseRef, l Literal) {
	s.traceBackwardSubsumption("subsumption_resolution", 441, func(w io.Writer) error {
		a1, err := s.kernel.clauseReader(c1)
		if err != nil {
			return err
		}
		left, err := satTraceClause(a1, c1)
		if err != nil {
			return err
		}
		right, err := satTraceClause(s.lifecycle.propagation.clauses, c2)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, left+" sub-ref("+l.String()+") "+right+"\n")
		return err
	})
}

func (s *satSimplifier) traceSubsumptionResult(c2 StoredClauseRef) {
	s.traceBackwardSubsumption("subsumption_resolution", 444, func(w io.Writer) error {
		clause, err := satTraceClause(s.lifecycle.propagation.clauses, c2)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, "result: "+clause+"\n")
		return err
	})
}
