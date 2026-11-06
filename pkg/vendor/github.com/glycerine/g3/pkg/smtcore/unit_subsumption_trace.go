package smtcore

// Source: src/smt/tactic/unit_subsumption_tactic.cpp:76,105.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (t *UnitSubsumptionTactic) traceRemoval(index uint32) error {
	tags := t.manager.BorrowSourceTraceTags()
	err := tags.EmitConditional("unit_subsumption_tactic", "prune_clause", "src/smt/tactic/unit_subsumption_tactic.cpp", 105,
		func() bool { return true }, func(out io.Writer) error {
			_, err := fmt.Fprintf(out, "Removing clause %d\n", index)
			return err
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func (t *UnitSubsumptionTactic) traceResult(goal *tactic.Goal) (result error) {
	tags := t.manager.BorrowSourceTraceTags()
	if !tags.ShouldTrace("unit_subsumption_tactic") {
		return nil
	}
	globals := t.solver.kernel.paramGlobals
	if globals != nil && uparams.BorrowProcessGlobalParamsService() != globals {
		previous, err := uparams.BindProcessGlobalParamsService(globals)
		if err != nil {
			return err
		}
		defer func() {
			_, err := uparams.BindProcessGlobalParamsService(previous)
			result = errors.Join(result, err)
		}()
	}
	var bodyErr error
	err := tags.EmitConditional("unit_subsumption_tactic", "reduce_core", "src/smt/tactic/unit_subsumption_tactic.cpp", 76,
		func() bool { return true }, func(out io.Writer) error {
			// Both source goal references point at the same modified goal.
			w := smtTacticTraceOutput{Writer: out}
			bodyErr = goal.Display(w)
			if bodyErr == nil {
				bodyErr = goal.Display(w)
			}
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
