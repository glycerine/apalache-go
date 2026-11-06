package bv

// Source: src/tactic/bv/bv_bounds_tactic.cpp/.h mk_bv_bounds_tactic.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/core"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func NewBVBoundsTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*core.ContextSimplifyTactic, error) {
	s := NewBVBoundsSimplifier(m, c)
	t, e := core.NewContextSimplifyTacticWithSimplifier(m, s, c, g, p, services)
	if e != nil {
		return nil, e
	}
	return t, nil
}

// NewBVBoundsDomTactic constructs the registered propagate-bv-bounds-new
// tactic, whose bounds state is scoped by the dominator traversal.
func NewBVBoundsDomTactic(m *ast.ASTManager, c *uparams.ParamsContext, p *uparams.ParamsRef) (*core.DomSimplifyTactic, error) {
	if m == nil || c == nil {
		return nil, uparams.ErrParamsOwner
	}
	s := NewBVBoundsSimplifier(m, c)
	t, err := core.NewDomSimplifyTacticWithSimplifier(m, s, p)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func MakeBVBoundsDomTactic(m *ast.ASTManager, c *uparams.ParamsContext, p *uparams.ParamsRef) (tactic.Tactic, error) {
	t, err := NewBVBoundsDomTactic(m, c, p)
	if err != nil {
		return nil, err
	}
	w, err := tactic.NewCleanTactic(t)
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return w, nil
}

func MakeBVBoundsTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	t, e := NewBVBoundsTactic(m, c, g, p, services)
	if e != nil {
		return nil, e
	}
	clean, e := tactic.NewCleanTactic(t)
	if e != nil {
		return nil, errors.Join(e, t.Close())
	}
	return clean, nil
}
