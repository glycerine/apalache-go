package sat

// Source: src/sat/sat_solver.cpp:3287,3289,3297,3307,3366,3420,3450,3516,3538,3585; retained util/trace sites.

import (
	"io"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *Solver) traceConsequenceRecord(function string, line int, body func(io.Writer) error) {
	tags := s.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("sat", function, "src/sat/sat_solver.cpp", line,
		func() bool { return true }, body)
}

func (s *Solver) traceConsequenceInconsistent() {
	s.traceConsequenceRecord("get_consequences", 3420, func(w io.Writer) error {
		if _, err := io.WriteString(w, "inconsistent\n"); err != nil {
			return err
		}
		return s.traceUserDisplayInto(w)
	})
}

func (s *Solver) traceConsequenceUnsat() {
	s.traceConsequenceRecord("get_consequences", 3450, func(w io.Writer) error {
		_, err := io.WriteString(w, "unsat\n")
		return err
	})
}

func (s *Solver) traceConsequenceCore(line int) {
	s.traceConsequenceRecord("fixup_consequence_core", line, func(w io.Writer) error {
		n, err := s.conflict.core.Size()
		if err != nil {
			return err
		}
		var b strings.Builder
		for i := uint32(0); i < n; i++ {
			v, err := s.conflict.core.Read(i)
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

func (s *Solver) traceConsequenceAntecedents(l Literal, row *satSolverIndexSet) {
	s.traceConsequenceRecord("fixup_consequence_core", 3289, func(w io.Writer) error {
		if err := row.open(); err != nil {
			return err
		}
		var b strings.Builder
		b.WriteString(l.String())
		b.WriteString(": ")
		for key := range row.keys.All() {
			b.WriteString(Literal(key).String())
			b.WriteByte(' ')
		}
		b.WriteByte('\n')
		_, _ = io.WriteString(w, b.String())
		return nil
	})
}

func (s *Solver) traceConsequenceAssumptions(function string, line int, asms satConsequenceInput) {
	s.traceConsequenceRecord(function, line, func(w io.Writer) error {
		n, err := asms.size()
		if err != nil {
			return err
		}
		var b strings.Builder
		for i := uint32(0); i < n; i++ {
			v, err := asms.read(i)
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

func (s *Solver) traceExtractFixedLiteral(l Literal) {
	s.traceConsequenceRecord("extract_fixed_consequences", 3516, func(w io.Writer) error {
		value, err := s.Variables.valueLiteral(l)
		if err != nil {
			return err
		}
		level, err := s.Variables.levelVar(l.variable())
		if err != nil {
			return err
		}
		_, _ = io.WriteString(w, "extract: "+l.String()+" "+value.String()+" "+strconv.FormatUint(uint64(level), 10)+"\n")
		return nil
	})
}

func (s *Solver) traceConsequenceReason(l Literal, j satJustification) {
	s.traceConsequenceRecord("extract_assumptions", 3538, func(w io.Writer) error {
		_, _ = io.WriteString(w, l.String()+" "+satJustificationTraceString(j)+"\n")
		return nil
	})
}

func (s *Solver) traceConsequenceReasonResult(l Literal, row *satSolverIndexSet) {
	s.traceConsequenceRecord("extract_assumptions", 3585, func(w io.Writer) error {
		if err := row.open(); err != nil {
			return err
		}
		var b strings.Builder
		b.WriteString(l.String())
		b.WriteString(": ")
		for key := range row.keys.All() {
			b.WriteString(Literal(key).String())
			b.WriteByte(' ')
		}
		b.WriteByte('\n')
		_, _ = io.WriteString(w, b.String())
		return nil
	})
}
