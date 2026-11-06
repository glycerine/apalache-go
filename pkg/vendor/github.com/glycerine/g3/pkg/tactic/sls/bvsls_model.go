package sls

// Source: src/tactic/sls/bvsls_opt_engine.cpp:165-193 and
// src/tactic/sls/bvsls_opt_engine.h:39-60.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (o *BVOptimizer) saveModel(score *numeral.Int) (result error) {
	if err := o.open(); err != nil {
		return err
	}
	hard, err := o.hard.tracker.GetModel()
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		if !keep {
			result = errors.Join(result, hard.Close())
		}
	}()
	objective, err := o.objective.GetModel()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, objective.Close()) }()
	count, err := objective.Core().NumConstants()
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		decl, err := objective.Core().Constant(i)
		if err != nil {
			return err
		}
		value, err := objective.Core().ConstantInterpretation(decl)
		if err != nil {
			return err
		}
		has, err := hard.Core().HasConstantInterpretation(decl)
		if err != nil {
			return err
		}
		if has {
			current, x := hard.Core().ConstantInterpretation(decl)
			if x != nil {
				return x
			}
			if current != value {
				return ErrBVSLSOptimizer
			}
			continue
		}
		if err = hard.Core().RegisterConstant(decl, value); err != nil {
			return err
		}
	}
	if o.bestModel != nil {
		if err = o.bestModel.Close(); err != nil {
			return err
		}
	}
	if err = o.hard.numbers.Copy(&o.bestScore, score); err != nil {
		return err
	}
	o.bestModel, keep = hard, true
	return nil
}

func (o *BVOptimizer) optimumAST() (ast.ASTHandle, error) {
	if err := o.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if o.bestModel == nil || o.objectiveExpr.IsNull() {
		return ast.ASTHandle{}, ErrBVSLSOptimizer
	}
	sort, err := o.hard.manager.GetSort(o.objectiveExpr)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return o.objective.ValueAST(sort, &o.bestScore)
}

// GetModel returns an owned copy so the optimizer keeps the best model across
// later restarts and calls, matching model_ref source lifetime.
func (o *BVOptimizer) GetModel() (*model.Model, error) {
	if err := o.open(); err != nil {
		return nil, err
	}
	if o.bestModel == nil {
		return nil, ErrBVSLSOptimizer
	}
	return o.bestModel.Copy()
}
