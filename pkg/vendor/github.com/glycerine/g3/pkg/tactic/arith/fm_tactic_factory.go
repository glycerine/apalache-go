package arith

// Source: src/tactic/arith/fm_tactic.cpp:1685-1693.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/core"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func MakeFMTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (result tactic.Tactic, resultErr error) {
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	local, err := c.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.Del(&local)) }()
	if p != nil {
		if err = c.Assign(&local, p); err != nil {
			return nil, err
		}
	}
	for _, name := range []string{"arith_lhs", "elim_and", "som"} {
		if err = c.SetBoolName(&local, &name, true); err != nil {
			return nil, err
		}
	}
	simple, err := core.MakeSimplifyTactic(m, c, g, &local)
	if err != nil {
		return nil, err
	}
	if err = simple.IncRef(); err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, simple.DecRef()) }()
	configured, err := tactic.NewUsingParamsTactic(c, simple, &local)
	if err != nil {
		return nil, err
	}
	if err = configured.IncRef(); err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, configured.DecRef()) }()
	child, err := NewFMTactic(m, c, g, p, services)
	if err != nil {
		return nil, err
	}
	clean, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	if err = clean.IncRef(); err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, clean.DecRef()) }()
	return tactic.NewAndThenTactic(configured, clean)
}
