package bv

// Source: src/tactic/bv/bv_slice_tactic.h mk_bv_slice_tactic.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func MakeBVSliceTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	t, e := NewBVSliceTactic(m, c, g, p, services)
	if e != nil {
		return nil, e
	}
	clean, e := tactic.NewCleanTactic(t)
	if e != nil {
		return nil, errors.Join(e, t.Close())
	}
	return clean, nil
}
