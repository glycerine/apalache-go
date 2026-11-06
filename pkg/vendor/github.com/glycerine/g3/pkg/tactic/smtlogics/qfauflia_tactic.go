package smtlogics

// Source: src/tactic/smtlogics/qfauflia_tactic.cpp/.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/core"
	_ "github.com/glycerine/g3/pkg/tactic/smtlogics/params"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// NewQFAUFLIATactic constructs the source preamble and SMT backend with their
// distinct fixed parameter scopes. The caller acquires the returned tactic.
func NewQFAUFLIATactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (result tactic.Tactic, err error) {
	c, g := config.Params, config.Globals
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	var owners []tactic.Tactic
	defer func() {
		for i := len(owners) - 1; i >= 0; i-- {
			err = errors.Join(err, owners[i].DecRef())
		}
		if err != nil && result != nil {
			if e := result.IncRef(); e != nil {
				err = errors.Join(err, e)
			} else {
				err = errors.Join(err, result.DecRef())
			}
			result = nil
		}
	}()
	keep := func(value tactic.Tactic, cause error) (tactic.Tactic, error) {
		if cause != nil {
			return nil, cause
		}
		if cause = value.IncRef(); cause != nil {
			return nil, cause
		}
		owners = append(owners, value)
		return value, nil
	}
	mainParams, err := c.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, c.Del(&mainParams)) }()
	for _, name := range []string{"elim_and", "som", "sort_store"} {
		if err = c.SetBoolName(&mainParams, &name, true); err != nil {
			return nil, err
		}
	}
	solverParams, err := c.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, c.Del(&solverParams)) }()
	name := "array.simplify"
	if err = c.SetBoolName(&solverParams, &name, false); err != nil {
		return nil, err
	}
	first, err := keep(core.MakeSimplifyTactic(m, c, g, nil))
	if err != nil {
		return nil, err
	}
	propagate, err := keep(core.NewPropagateValuesTactic(m, c, g, nil))
	if err != nil {
		return nil, err
	}
	solve, err := keep(core.MakeSolveEquationsTactic(m, c, g, nil, config.ModelEvaluator))
	if err != nil {
		return nil, err
	}
	eliminate, err := keep(core.MakeEliminateUnconstrainedTactic(m, c, g, nil, config.ModelEvaluator))
	if err != nil {
		return nil, err
	}
	last, err := keep(core.MakeSimplifyTactic(m, c, g, nil))
	if err != nil {
		return nil, err
	}
	preamble, err := keep(tactic.NewAndThenTactic(first, propagate, solve, eliminate, last))
	if err != nil {
		return nil, err
	}
	preamble, err = keep(tactic.NewUsingParamsTactic(c, preamble, &mainParams))
	if err != nil {
		return nil, err
	}
	backend, err := keep(smtcore.NewSMTTactic(m, nil, config))
	if err != nil {
		return nil, err
	}
	backend, err = keep(tactic.NewUsingParamsTactic(c, backend, &solverParams))
	if err != nil {
		return nil, err
	}
	result, err = tactic.NewAndThenTactic(preamble, backend)
	if err != nil {
		return nil, err
	}
	if p == nil {
		empty := c.EmptyValue()
		p = &empty
	}
	err = result.UpdateParams(p)
	return result, err
}
