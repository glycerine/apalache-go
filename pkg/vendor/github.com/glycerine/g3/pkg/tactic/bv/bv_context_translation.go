package bv

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// TranslateWithServices creates the source's fresh reduction implementation
// with the destination manager and its services.
func (t *BVSizeReductionTactic) TranslateWithServices(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	next, err := NewBVSizeReductionTactic(m, c, g, services)
	if err != nil {
		return nil, err
	}
	next.trace = t.trace
	return next, nil
}

// TranslateWithServices recreates quantified bit-vector elimination using
// the target cooperation service and a translated parameter snapshot.
func (t *ElimSmallBVTactic) TranslateWithServices(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, cooperation *cooperate.Cooperation) (result tactic.Tactic, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	p, err := t.context.TranslateTo(&t.params, c)
	if err != nil {
		return nil, err
	}
	var next *ElimSmallBVTactic
	defer func() {
		resultErr = errors.Join(resultErr, c.Del(&p))
		if resultErr != nil && next != nil {
			resultErr = errors.Join(resultErr, next.Close())
			result = nil
		}
	}()
	next, err = NewElimSmallBVTactic(m, c, g, &p, cooperation)
	if err != nil {
		return nil, err
	}
	return next, nil
}

// TranslateWithServices creates a fresh array rewriter with target-owned
// model-converter services.
func (t *BVArray2UFTactic) TranslateWithServices(m *ast.ASTManager, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if m == nil || services.Context == nil || services.Globals == nil || services.Globals.Context != services.Context || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := services.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	next, err := NewBVArray2UFTactic(m, nil, services)
	if err != nil {
		return nil, err
	}
	next.trace = t.trace
	return next, nil
}

// TranslateWithServices copies the bound checker's parameter snapshot and
// creates independent target-owned rewriting and statistics state.
func (t *BVBoundCheckTactic) TranslateWithServices(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, services model.ModelEvaluatorConfig) (result tactic.Tactic, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	p, err := t.context.TranslateTo(&t.params, c)
	if err != nil {
		return nil, err
	}
	var next *BVBoundCheckTactic
	defer func() {
		resultErr = errors.Join(resultErr, c.Del(&p))
		if resultErr != nil && next != nil {
			resultErr = errors.Join(resultErr, next.Close())
			result = nil
		}
	}()
	next, err = NewBVBoundCheckTactic(m, c, g, &p, services)
	if err != nil {
		return nil, err
	}
	if err = next.BindTrace(t.trace); err != nil {
		return nil, err
	}
	return next, nil
}
