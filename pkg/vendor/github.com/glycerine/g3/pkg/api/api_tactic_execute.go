package api

// Source: src/api/api_tactic.cpp:400-428, _tactic_apply.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/control"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (t *APITactic) execute(goal *tactic.Goal, result *tactic.TacticResult, params *uparams.ParamsContext, local *uparams.ParamsRef) (resultErr error) {
	timeoutKey, ctrlCKey := "timeout", "ctrl_c"
	timeout, err := params.GetUIntName(local, &timeoutKey, math.MaxUint32)
	if err != nil {
		return err
	}
	ctrlC, err := params.GetBoolName(local, &ctrlCKey, false)
	if err != nil {
		return err
	}
	if err = t.tactic.UpdateParams(local); err != nil {
		return err
	}
	context := &SMTContext{smtContextState: t.owner}
	manager, err := context.Manager()
	if err != nil {
		return err
	}
	handler := newSMTContextCancelHandler(manager.ResourceLimit())
	scope, err := newSMTContextInterruptScope(context, handler)
	if err != nil {
		handler.close()
		return err
	}
	defer func() {
		resultErr = errors.Join(resultErr, scope.Close())
		handler.close()
	}()
	interrupt, err := control.NewScopedCtrlC(handler, false, ctrlC)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, interrupt.Close()) }()
	timer, err := control.NewScopedTimer(timeout, handler)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, timer.Close()) }()
	return tactic.Exec(t.tactic, goal, result)
}
