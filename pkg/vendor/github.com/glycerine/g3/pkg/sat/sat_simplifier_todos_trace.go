package sat

// Source: src/sat/sat_simplifier.cpp:815,845,863,870; retained util/trace sites.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *satSimplifier) traceSubsumeTodo(line int, body func(io.Writer) error) {
	tags := s.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("subsumption", "subsume", "src/sat/sat_simplifier.cpp", line,
		func() bool { return true }, body)
}

func (s *satSimplifier) traceSubsumptionTodoSize() {
	s.traceSubsumeTodo(815, func(w io.Writer) error {
		_, err := io.WriteString(w, "sub_todo size: "+strconv.FormatUint(uint64(s.subTodo.Size()), 10)+"\n")
		return err
	})
}

func (s *satSimplifier) traceSubsumptionTodoClause(line int, prefix string, c StoredClauseRef) {
	s.traceSubsumeTodo(line, func(w io.Writer) error {
		clause, err := satTraceClause(s.lifecycle.propagation.clauses, c)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, prefix+clause+"\n")
		return err
	})
}
