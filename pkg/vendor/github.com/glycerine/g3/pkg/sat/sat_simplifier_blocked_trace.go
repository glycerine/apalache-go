package sat

// Source: src/sat/sat_simplifier.cpp:951,967,1008,1079; retained util/trace sites.

import (
	"io"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (s *satSimplifier) traceBlocked(line int, body func(io.Writer) error) {
	tags := s.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("blocked_clause", "process", "src/sat/sat_simplifier.cpp", line,
		func() bool { return true }, body)
}

func (s *satSimplifier) traceBlockedProcessing(l Literal) {
	s.traceBlocked(951, func(w io.Writer) error {
		_, err := io.WriteString(w, "processing: "+l.String()+"\n")
		return err
	})
}

func (s *satSimplifier) traceBlockedClause(c StoredClauseRef) {
	s.traceBlocked(967, func(w io.Writer) error {
		clause, err := satTraceClause(s.lifecycle.propagation.clauses, c)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, "new blocked clause: "+clause+"\n")
		return err
	})
}

func (s *satSimplifier) traceBlockedBinary(other, selected Literal) {
	s.traceBlocked(1008, func(w io.Writer) error {
		_, err := io.WriteString(w, "new blocked clause: "+other.String()+" "+selected.String()+"\n")
		return err
	})
}

func (s *satSimplifier) traceBlockedStart() {
	if s.cleaner == nil || s.cleaner.solver == nil {
		return
	}
	tags := s.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("blocked_clause_bug", "elim_blocked_clauses", "src/sat/sat_simplifier.cpp", 1079,
		func() bool { return true }, func(w io.Writer) error {
			solver := s.cleaner.solver
			n, err := solver.Variables.trail.Size(solver.Variables.trailRef)
			if err != nil {
				return err
			}
			// The source trace stream is outside the solver's custom memory budget.
			out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: arena.NewBudget(^uint64(0))})
			if err != nil {
				return err
			}
			defer out.Close()
			if err = out.AppendCString("trail: "); err != nil {
				return err
			}
			if err = out.AppendUint32(n); err != nil {
				return err
			}
			if err = out.AppendCString("\n"); err != nil {
				return err
			}
			if err = solver.displayWatches(out); err != nil {
				return err
			}
			if err = solver.displayInto(out); err != nil {
				return err
			}
			body, err := out.String()
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, body)
			return err
		})
}
