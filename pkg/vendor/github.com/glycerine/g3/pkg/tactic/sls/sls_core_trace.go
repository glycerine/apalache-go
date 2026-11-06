package sls

// Source: src/tactic/sls/sls_tactic.cpp:70,77.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// Source ostream writes become no-ops after a stream failure, while later
// trace operands still execute.
type slsTraceWriter struct{ io.Writer }

func (w slsTraceWriter) Write(p []byte) (int, error) {
	_, _ = w.Writer.Write(p)
	return len(p), nil
}

func (t *CoreTactic) traceGoal(goal *tactic.Goal, line int) (result error) {
	tags := t.manager.BorrowSourceTraceTags()
	if !tags.ShouldTrace("sls") {
		return nil
	}
	if uparams.BorrowProcessGlobalParamsService() != t.globals {
		previous, err := uparams.BindProcessGlobalParamsService(t.globals)
		if err != nil {
			return err
		}
		defer func() {
			_, err := uparams.BindProcessGlobalParamsService(previous)
			result = errors.Join(result, err)
		}()
	}
	var bodyErr error
	err := tags.EmitConditional("sls", "operator()", "src/tactic/sls/sls_tactic.cpp", line,
		func() bool { return true }, func(out io.Writer) error {
			bodyErr = goal.Display(slsTraceWriter{out})
			return bodyErr
		})
	if bodyErr != nil {
		return bodyErr
	}
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
