package sat

// Source: src/sat/sat_solver.cpp:1272,1274; retained util/trace TRACE sites.

import (
	"io"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *Solver) traceModelRecord(tag string, line int, body func(io.Writer) error) {
	tags := s.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional(tag, "mk_model", "src/sat/sat_solver.cpp", line,
		func() bool { return true }, body)
}

func (s *Solver) traceModelConverterBeforeApply() {
	s.traceModelRecord("sat_mc_bug", 1272, s.traceModelConverterInto)
}

func (s *Solver) traceCompletedModel(n uint32, model satSolverModelSpan) {
	s.traceModelRecord("sat", 1274, func(w io.Writer) error {
		var b strings.Builder
		for v := uint32(0); v < n; v++ {
			value, err := model.read(BoolVar(v))
			if err != nil {
				return err
			}
			b.WriteString(strconv.FormatUint(uint64(v), 10))
			b.WriteString(": ")
			b.WriteString(value.String())
			b.WriteByte('\n')
		}
		_, _ = io.WriteString(w, b.String())
		return nil
	})
}
