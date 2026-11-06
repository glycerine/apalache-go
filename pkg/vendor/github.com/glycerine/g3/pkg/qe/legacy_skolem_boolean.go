package qe

// Source: src/muz/spacer/spacer_legacy_mbp.cpp Boolean substitution branch.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

func (p *LegacyProjection) substituteSkolemBooleans(md *model.ModelCore, formula ast.ASTRef, variables []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	evaluator, err := newLegacyModelEvaluator(md, model.ModelEvaluatorConfig{
		Context: p.config.Context, Globals: p.config.Globals, Memory: p.manager.NumeralService().RuntimeOwner(), Cooperation: p.config.Cooperation, Trace: p.config.Trace,
	})
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	if err = evaluator.SetModelCompletion(true); err != nil {
		return ast.ASTRef{}, err
	}
	replace, err := rewriter.NewExprSafeReplace(p.manager)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, replace.Close()) }()
	values := make([]ast.ASTRef, 0, len(variables))
	defer func() { resultErr = errors.Join(resultErr, truncateLiteVector(p.manager, &values, 0)) }()
	for _, variable := range variables {
		value, x := evaluator.EvaluateValue(variable)
		if x != nil {
			return ast.ASTRef{}, x
		}
		values = append(values, value)
		handle, x := p.manager.RefGet(value)
		if x == nil {
			x = replace.Insert(variable, handle)
		}
		if x != nil {
			return ast.ASTRef{}, x
		}
	}
	result, err = p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	body, err := p.manager.RefGet(formula)
	if err == nil {
		err = replace.Apply(body, result)
	}
	if err != nil {
		return ast.ASTRef{}, errors.Join(err, p.manager.DelRef(&result))
	}
	return result, nil
}
