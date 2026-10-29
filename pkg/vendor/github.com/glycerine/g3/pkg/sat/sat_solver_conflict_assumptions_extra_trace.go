package sat

// Source: src/sat/sat_solver.cpp:1825,1892,1932,1940; retained util/trace TRACE sites.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *satSolverConflictResolution) traceAssumptionCoreRecord(function string, line int, body func(io.Writer) error) {
	tags := s.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("sat", function, "src/sat/sat_solver.cpp", line,
		func() bool { return true }, body)
}

func (s *satSolverConflictResolution) traceUnsatCoreAntecedent(l Literal, marked bool) {
	s.traceAssumptionCoreRecord("process_antecedent_for_unsat_core", 1825, func(w io.Writer) error {
		flag := "-"
		if marked {
			flag = "+"
		}
		_, _ = io.WriteString(w, l.String()+" "+flag+"\n")
		return nil
	})
}

func (s *satSolverConflictResolution) traceInitNotLiteral(l Literal) {
	s.traceAssumptionCoreRecord("resolve_conflict_for_init", 1892, func(w io.Writer) error {
		_, _ = io.WriteString(w, "not_l: "+l.String()+"\n")
		return nil
	})
}

func (s *satSolverConflictResolution) traceInitLemmaState() {
	// An isolated conflict owner has no enclosing solver to display. Production
	// conflict resolution always has the concrete solver bound.
	if s.solver == nil {
		return
	}
	s.traceAssumptionCoreRecord("resolve_conflict_for_init", 1932, func(w io.Writer) error {
		if _, err := io.WriteString(w, "lemma: "); err != nil {
			return err
		}
		n, err := s.analysis.lemma.Size()
		if err != nil {
			return err
		}
		for i := uint32(0); i < n; i++ {
			l, err := s.analysis.lemma.Read(i)
			if err != nil {
				return err
			}
			if i != 0 {
				if _, err := io.WriteString(w, " "); err != nil {
					return err
				}
			}
			if _, err := io.WriteString(w, Literal(l).String()); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
		if err := s.solver.traceUserDisplayInto(w); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "assignment:\n"); err != nil {
			return err
		}
		return s.solver.traceUserAssignmentInto(w)
	})
}

func (s *satSolverConflictResolution) traceInitTrail() {
	s.traceAssumptionCoreRecord("resolve_conflict_for_init", 1940, func(w io.Writer) error {
		v := s.analysis.propagation.variables
		n, err := v.trail.Size(v.trailRef)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(w, "Trail: "); err != nil {
			return err
		}
		for i := uint32(0); i < n; i++ {
			l, err := v.trail.Read(v.trailRef, i)
			if err != nil {
				return err
			}
			if i != 0 {
				if _, err := io.WriteString(w, " "); err != nil {
					return err
				}
			}
			if _, err := io.WriteString(w, l.String()); err != nil {
				return err
			}
		}
		_, err = io.WriteString(w, "\n")
		return err
	})
}
