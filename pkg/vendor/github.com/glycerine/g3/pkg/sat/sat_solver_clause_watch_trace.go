package sat

// Source: src/sat/sat_solver.cpp:419; retained util/trace CTRACE site.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *satSolverClauseState) traceLearnedWatchValue(l Literal) error {
	tags := s.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	var valueErr error
	_ = tags.EmitConditional("sat", "select_learned_watch_lit", "src/sat/sat_solver.cpp", 419,
		func() bool {
			value, err := s.value(l)
			valueErr = err
			return err == nil && value != lbool.False
		}, func(w io.Writer) error {
			value, err := s.value(l)
			if err != nil {
				valueErr = err
				return err
			}
			_, _ = io.WriteString(w, l.String()+":="+value.String())
			return nil
		})
	return valueErr
}
