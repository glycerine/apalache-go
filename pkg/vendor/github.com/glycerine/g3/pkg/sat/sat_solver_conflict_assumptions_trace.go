package sat

// Source: src/sat/sat_solver.cpp:1143,1151-1157; retained util/trace TRACE sites.

import (
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *satSolverConflictResolution) traceAssumptionsRecord(line int, body func(io.Writer) error) {
	tags := s.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("sat", "reinit_assumptions", "src/sat/sat_solver.cpp", line,
		func() bool { return true }, body)
}

func (s *satSolverConflictResolution) traceReinitAssumptions() {
	s.traceAssumptionsRecord(1143, func(w io.Writer) error {
		n, err := s.assumptions.Size()
		if err != nil {
			return err
		}
		var b strings.Builder
		for i := uint32(0); i < n; i++ {
			v, err := s.assumptions.Read(i)
			if err != nil {
				return err
			}
			if i != 0 {
				b.WriteByte(' ')
			}
			b.WriteString(Literal(v).String())
		}
		b.WriteByte('\n')
		_, _ = io.WriteString(w, b.String())
		return nil
	})
}

func (s *satSolverConflictResolution) traceReinitAntecedents() {
	s.traceAssumptionsRecord(1151, func(w io.Writer) error {
		if s.solver == nil || s.solver.antecedents == nil {
			return nil
		}
		if err := s.solver.antecedents.open(); err != nil {
			return err
		}
		n, err := s.assumptions.Size()
		if err != nil {
			return err
		}
		for i := uint32(0); i < n; i++ {
			raw, err := s.assumptions.Read(i)
			if err != nil {
				return err
			}
			lit := Literal(raw)
			row := s.solver.antecedents.entries[uint32(lit.variable())]
			if row == nil {
				continue
			}
			if err = row.open(); err != nil {
				return err
			}
			_, _ = io.WriteString(w, lit.String()+": ")
			for key := range row.keys.All() {
				_, _ = io.WriteString(w, Literal(key).String()+" ")
			}
			_, _ = io.WriteString(w, "\n")
		}
		return nil
	})
}
