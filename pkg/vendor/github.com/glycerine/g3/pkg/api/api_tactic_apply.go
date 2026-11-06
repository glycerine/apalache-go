package api

// Source: src/api/api_tactic.cpp:439-459.

import (
	"errors"

	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (t *APITactic) Apply(goal *APIGoal) (*APITacticResult, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	globals, err := t.owner.command.GlobalParameters()
	if err != nil {
		return nil, err
	}
	empty := globals.Context.EmptyValue()
	return t.apply(goal, globals.Context, &empty)
}

func (t *APITactic) apply(goal *APIGoal, params *uparams.ParamsContext, local *uparams.ParamsRef) (result *APITacticResult, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if err := goal.open(); err != nil {
		return nil, err
	}
	if t.owner != goal.owner {
		return nil, ErrSMTContextOwner
	}
	working, err := tactic.NewGoalCopy(goal.goal)
	if err != nil {
		return nil, err
	}
	if err = working.IncRef(); err != nil {
		return nil, errors.Join(err, working.Close())
	}
	native := &tactic.TacticResult{}
	defer func() {
		resultErr = errors.Join(resultErr, native.Close(), working.DecRef())
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	if err = t.execute(working, native, params, local); err != nil {
		return nil, err
	}
	result = &APITacticResult{owner: t.owner, manager: goal.manager}
	result.self = result
	if native.ModelConverter != nil {
		if err = native.ModelConverter.IncRef(); err != nil {
			return result, err
		}
		result.modelConverter = native.ModelConverter
	}
	if native.ProofConverter != nil {
		if err = native.ProofConverter.IncRef(); err != nil {
			return result, err
		}
		result.proofConverter = native.ProofConverter
	}
	if err = result.core.Set(goal.manager, native.Core); err != nil {
		return result, err
	}
	for i := range native.Goals {
		subgoal, err := native.AcquireGoalReference(uint32(i))
		if err != nil {
			return result, err
		}
		result.goals = append(result.goals, subgoal)
	}

	return result, nil
}
