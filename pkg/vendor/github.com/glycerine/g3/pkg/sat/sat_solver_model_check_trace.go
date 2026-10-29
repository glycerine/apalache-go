package sat

// Source: src/sat/sat_solver.cpp:1299-1342; retained util/trace TRACE sites.

import (
	"io"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *Solver) traceCheckModelRecord(line int, body func(io.Writer) error) {
	tags := s.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("sat", "check_model", "src/sat/sat_solver.cpp", line,
		func() bool { return true }, body)
}

// Source sat::model stream insertion skips undefined entries.
func satTraceModelValues(model satSolverModelSpan) (string, error) {
	n := uint32(len(model.slice))
	if model.column != nil {
		var err error
		n, err = model.column.Size()
		if err != nil {
			return "", err
		}
	}
	var b strings.Builder
	for i := uint32(0); i < n; i++ {
		v, err := model.read(BoolVar(i))
		if err != nil {
			return "", err
		}
		if v == lbool.Undef {
			continue
		}
		if b.Len() != 0 {
			b.WriteByte(' ')
		}
		if v == lbool.False {
			b.WriteByte('-')
		}
		b.WriteString(strconv.FormatUint(uint64(i), 10))
	}
	return b.String(), nil
}

func (s *Solver) traceCheckModelContext(w io.Writer, model satSolverModelSpan) error {
	n, err := s.Variables.trail.Size(s.Variables.trailRef)
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("trail: ")
	for i := uint32(0); i < n; i++ {
		l, err := s.Variables.trailRead(i)
		if err != nil {
			return err
		}
		if i != 0 {
			b.WriteByte(' ')
		}
		b.WriteString(l.String())
	}
	values, err := satTraceModelValues(model)
	if err != nil {
		return err
	}
	b.WriteString("\nmodel: ")
	b.WriteString(values)
	b.WriteByte('\n')
	_, _ = io.WriteString(w, b.String())
	return s.traceModelConverterInto(w)
}

func (s *Solver) traceModelFailedClause(c StoredClauseRef, model satSolverModelSpan) {
	s.traceCheckModelRecord(1299, func(w io.Writer) error {
		clause, err := satTraceClause(s.Clauses, c)
		if err != nil {
			return err
		}
		n, err := s.conflict.assumptions.Size()
		if err != nil {
			return err
		}
		var b strings.Builder
		b.WriteString("failed: ")
		b.WriteString(clause)
		b.WriteString("\nassumptions: ")
		for i := uint32(0); i < n; i++ {
			raw, err := s.conflict.assumptions.Read(i)
			if err != nil {
				return err
			}
			if i != 0 {
				b.WriteByte(' ')
			}
			b.WriteString(Literal(raw).String())
		}
		b.WriteByte('\n')
		_, _ = io.WriteString(w, b.String())
		return s.traceCheckModelContext(w, model)
	})
}

func (s *Solver) traceModelFailedBinary(l, l2 Literal, learned bool) {
	s.traceCheckModelRecord(1322, func(w io.Writer) error {
		flag := "0"
		if learned {
			flag = "1"
		}
		_, _ = io.WriteString(w, "failed binary: "+l.String()+" "+l2.String()+" learned: "+flag+"\n")
		return s.traceModelConverterInto(w)
	})
}

func (s *Solver) traceModelFailedAssumption(l Literal, model satSolverModelSpan) {
	s.traceCheckModelRecord(1331, func(w io.Writer) error {
		_, _ = io.WriteString(w, l.String()+" does not model check\n")
		return s.traceCheckModelContext(w, model)
	})
}

func (s *Solver) traceModelFailedConverter(model satSolverModelSpan) {
	s.traceCheckModelRecord(1342, func(w io.Writer) error {
		values, err := satTraceModelValues(model)
		if err != nil {
			return err
		}
		_, _ = io.WriteString(w, "model: "+values+"\n")
		return s.traceModelConverterInto(w)
	})
}
