package sat

// Source: src/sat/sat_solver.cpp:1695,1718,1726,1788,1795,1812.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *satSolverConflictResolution) traceConflictRecord(tag string, line int, body func(io.Writer) error) {
	tags := s.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional(tag, "resolve_conflict_core", "src/sat/sat_solver.cpp", line,
		func() bool { return true }, body)
}

func (s *satSolverConflictResolution) traceConflictDetected(notLiteral Literal) {
	s.traceConflictRecord("sat", 1695, func(w io.Writer) error {
		body := "conflict detected at level " + strconv.FormatUint(uint64(s.conflictLevel), 10) + " for "
		if notLiteral == DefaultLiteral() {
			body += "null literal\n"
		} else {
			body += notLiteral.String() + "\n"
		}
		_, err := io.WriteString(w, body)
		return err
	})
}

func (s *satSolverConflictResolution) traceConflictNotLiteral(l Literal) {
	s.traceConflictRecord("sat_conflict", 1718, func(w io.Writer) error {
		_, err := io.WriteString(w, "not_l: "+l.String()+"\n")
		return err
	})
}

func (s *satSolverConflictResolution) traceProcessingConsequent(l Literal, marks uint32, j satJustification) {
	s.traceConflictRecord("sat_conflict_detail", 1726, func(w io.Writer) error {
		_, err := io.WriteString(w, "processing consequent: "+l.String()+"\n"+
			"num_marks: "+strconv.FormatUint(uint64(marks), 10)+", js: "+satJustificationTraceString(j)+"\n")
		return err
	})
}

func (s *satSolverConflictResolution) traceNewLemma(afterMinimization bool) {
	line := 1788
	prefix := "new lemma size: "
	if afterMinimization {
		line = 1795
		prefix = "new lemma (after minimization) size: "
	}
	s.traceConflictRecord("sat_lemma", line, func(w io.Writer) error {
		n, err := s.analysis.lemma.Size()
		if err != nil {
			return err
		}
		if _, err = io.WriteString(w, prefix+strconv.FormatUint(uint64(n), 10)+"\n"); err != nil {
			return err
		}
		for i := uint32(0); i < n; i++ {
			raw, err := s.analysis.lemma.Read(i)
			if err != nil {
				return err
			}
			if i != 0 {
				if _, err = io.WriteString(w, " "); err != nil {
					return err
				}
			}
			if _, err = io.WriteString(w, Literal(raw).String()); err != nil {
				return err
			}
		}
		_, err = io.WriteString(w, "\n")
		return err
	})
}

func (s *satSolverConflictResolution) tracePostBackjumpDisplay() {
	// Production conflict resolution has an enclosing solver. Isolated owner
	// fixtures omit it and cannot render the solver-wide source display.
	if s.solver == nil {
		return
	}
	s.traceConflictRecord("sat_conflict_detail", 1812, func(w io.Writer) error {
		if err := s.solver.traceUserDisplayInto(w); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "assignment:\n"); err != nil {
			return err
		}
		return s.solver.traceUserAssignmentInto(w)
	})
}
