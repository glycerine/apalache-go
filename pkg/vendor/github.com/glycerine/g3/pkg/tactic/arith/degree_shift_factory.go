package arith

// Source: src/tactic/arith/degree_shift_tactic.cpp, mk_degree_shift_tactic.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/core"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func MakeDegreeShiftTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, services model.ModelEvaluatorConfig) (result tactic.Tactic, err error) {
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
	keep := func(t tactic.Tactic, e error) (tactic.Tactic, error) {
		if e != nil {
			return nil, e
		}
		if e = t.IncRef(); e != nil {
			return nil, e
		}
		owners = append(owners, t)
		return t, nil
	}
	params, err := c.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, c.Del(&params)) }()
	name := "mul_to_power"
	if err = c.SetBoolName(&params, &name, true); err != nil {
		return nil, err
	}
	simplify, err := keep(core.MakeSimplifyTactic(m, c, g, nil))
	if err != nil {
		return nil, err
	}
	simplify, err = keep(tactic.NewUsingParamsTactic(c, simplify, &params))
	if err != nil {
		return nil, err
	}
	shift, err := keep(NewDegreeShiftTactic(m, services))
	if err != nil {
		return nil, err
	}
	shift, err = keep(tactic.NewCleanTactic(shift))
	if err != nil {
		return nil, err
	}
	return tactic.NewAndThenTactic(simplify, shift)
}
