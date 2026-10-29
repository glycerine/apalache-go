package smtlogics

// Source: src/tactic/smtlogics/qfuf_tactic.cpp/.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/core"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// NewQFUFTactic constructs the retained simplification/equation-solving
// preamble, guarded symmetry reduction, and SMT backend in source order.
func NewQFUFTactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (result tactic.Tactic, err error) {
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
	local, err := c.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, c.Del(&local)) }()
	for _, name := range []string{"pull_cheap_ite", "local_ctx"} {
		if err = c.SetBoolName(&local, &name, true); err != nil {
			return nil, err
		}
	}
	name := "local_ctx_limit"
	if err = c.SetUIntName(&local, &name, 10000000); err != nil {
		return nil, err
	}
	first, err := keep(core.MakeSimplifyTactic(m, c, g, p))
	if err != nil {
		return nil, err
	}
	propagate, err := keep(core.NewPropagateValuesTactic(m, c, g, p))
	if err != nil {
		return nil, err
	}
	solve, err := keep(core.MakeSolveEquationsTactic(m, c, g, p, config.ModelEvaluator))
	if err != nil {
		return nil, err
	}
	second, err := keep(core.MakeSimplifyTactic(m, c, g, p))
	if err != nil {
		return nil, err
	}
	second, err = keep(tactic.NewUsingParamsTactic(c, second, &local))
	if err != nil {
		return nil, err
	}
	symmetry, err := keep(core.NewSymmetryReduceTactic(m, c, g))
	if err != nil {
		return nil, err
	}
	symmetry, err = keep(tactic.NewIfNoUnsatCoresTactic(symmetry))
	if err != nil {
		return nil, err
	}
	symmetry, err = keep(tactic.NewIfNoProofsTactic(symmetry))
	if err != nil {
		return nil, err
	}
	backend, err := keep(smtcore.NewSMTTactic(m, p, config))
	if err != nil {
		return nil, err
	}
	return tactic.NewAndThenTactic(first, propagate, solve, second, symmetry, backend)
}
