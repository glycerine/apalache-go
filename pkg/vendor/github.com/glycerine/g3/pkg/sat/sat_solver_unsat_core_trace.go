package sat

// Source: src/sat/sat_solver.cpp:1836,2000,2032,2063; retained util/trace TRACE sites.

import (
	"io"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *satSolverConflictResolution) traceUnsatCoreRecord(function string, line int, body func(io.Writer) error) {
	tags := s.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("sat", function, "src/sat/sat_solver.cpp", line,
		func() bool { return true }, body)
}

func (s *satSolverConflictResolution) traceUnsatCoreStart() {
	// The source method always belongs to a full solver. Isolated Go conflict
	// fixtures intentionally omit that outer owner and cannot display it.
	if s.solver == nil {
		return
	}
	s.traceUnsatCoreRecord("resolve_conflict_for_unsat_core", 2000, func(w io.Writer) error {
		if err := s.solver.traceUserDisplayInto(w); err != nil {
			return err
		}
		v := s.analysis.propagation.variables
		n, err := v.trail.Size(v.trailRef)
		if err != nil {
			return err
		}
		var b strings.Builder
		level := uint32(0)
		for i := uint32(0); i < n; i++ {
			l, err := v.trailRead(i)
			if err != nil {
				return err
			}
			next, err := v.levelVar(l.variable())
			if err != nil {
				return err
			}
			if level != next {
				level = next
				b.WriteString(strconv.FormatUint(uint64(level), 10))
				b.WriteString(": ")
			}
			b.WriteString(l.String())
			marked, err := s.analysis.marked(uint32(l.variable()))
			if err != nil {
				return err
			}
			if marked {
				b.WriteByte('*')
			}
			b.WriteByte(' ')
		}
		b.WriteByte('\n')
		_, _ = io.WriteString(w, b.String())
		return nil
	})
}

func (s *satSolverConflictResolution) traceUnsatCoreJustificationInto(w io.Writer, j satJustification) error {
	_, _ = io.WriteString(w, satJustificationTraceString(j))
	if j.Clause() {
		c, err := s.analysis.propagation.clauses.GetClause(j.ClauseOffset())
		if err != nil {
			return err
		}
		clause, err := satTraceClause(s.analysis.propagation.clauses, c)
		if err != nil {
			return err
		}
		_, _ = io.WriteString(w, clause)
	}
	return nil
}

func (s *satSolverConflictResolution) traceUnsatCoreNotLiteral(l Literal, j satJustification) {
	s.traceUnsatCoreRecord("resolve_conflict_for_unsat_core", 2032, func(w io.Writer) error {
		_, _ = io.WriteString(w, "not_l: "+l.String()+"\n")
		if err := s.traceUnsatCoreJustificationInto(w, j); err != nil {
			return err
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	})
}

func (s *satSolverConflictResolution) traceUnsatCoreLowerLevel(l Literal, level uint32) {
	s.traceUnsatCoreRecord("resolve_conflict_for_unsat_core", 2063, func(w io.Writer) error {
		_, _ = io.WriteString(w, l.String()+" at level "+strconv.FormatUint(uint64(level), 10)+"\n")
		return nil
	})
}

func (s *satSolverConflictResolution) traceUnsatCoreConsequent(l Literal, j satJustification) {
	s.traceUnsatCoreRecord("process_consequent_for_unsat_core", 1836, func(w io.Writer) error {
		name := l.String()
		if l == DefaultLiteral() {
			name = "null"
		}
		_, _ = io.WriteString(w, "processing consequent: "+name+"\njs kind: ")
		if err := s.traceUnsatCoreJustificationInto(w, j); err != nil {
			return err
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	})
}
