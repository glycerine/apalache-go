package sat

// Source: src/sat/sat_simplifier.cpp:644,654,660,664,670,676.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *satSimplifier) traceElimLit(line int, body func(io.Writer) error) {
	tags := s.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("elim_lit", "elim_lit", "src/sat/sat_simplifier.cpp", line,
		func() bool { return true }, body)
}

func (s *satSimplifier) traceElimLitClause(line int, prefix string, c StoredClauseRef) {
	s.traceElimLit(line, func(w io.Writer) error {
		clause, err := satTraceClause(s.lifecycle.propagation.clauses, c)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, prefix+clause+"\n")
		return err
	})
}

func (s *satSimplifier) traceElimLitText(line int, message string) {
	s.traceElimLit(line, func(w io.Writer) error {
		_, err := io.WriteString(w, message)
		return err
	})
}

func (s *satSimplifier) traceElimLitUnit(l Literal) {
	s.traceElimLitText(664, "clause became unit: "+l.String()+"\n")
}

func (s *satSimplifier) traceElimLitBinary(a, b Literal) {
	s.traceElimLitText(670, "clause became binary: "+a.String()+" "+b.String()+"\n")
}
