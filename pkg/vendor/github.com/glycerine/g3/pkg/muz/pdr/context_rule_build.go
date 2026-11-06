package pdr

// Source: context::init_rules transformer construction in
// src/muz/pdr/pdr_context.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func closeTransformers(order []ast.ASTHandle, transformers map[ast.ASTHandle]*PredicateTransformer) (result error) {
	for index := len(order) - 1; index >= 0; index-- {
		if transformer := transformers[order[index]]; transformer != nil {
			result = errors.Join(result, transformer.Close())
		}
	}
	return result
}

func (c *Context) buildRuleTransformers(plan *contextRulePlan) (_ map[ast.ASTHandle]*PredicateTransformer, resultErr error) {
	if plan == nil {
		return nil, ast.ErrASTDomain
	}
	transformers := make(map[ast.ASTHandle]*PredicateTransformer, len(plan.order))
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, closeTransformers(plan.order, transformers))
		}
	}()
	for _, declaration := range plan.order {
		transformer, err := NewPredicateTransformer(c.owner, declaration, c.cacheMode)
		if err != nil {
			return nil, err
		}
		transformers[declaration] = transformer
		if err = transformer.solver.SetUseFarkas(c.useFarkas && plan.nonBoolean); err != nil {
			return nil, err
		}
	}
	for _, declaration := range plan.order {
		if err := transformers[declaration].InitializeRules(transformers, plan.buckets[declaration]); err != nil {
			return nil, err
		}
	}
	committed = true
	return transformers, nil
}
