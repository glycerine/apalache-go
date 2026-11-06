package api

// Source: src/api/api_goal.cpp:154-165.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (g *APIGoal) Translate(target *SMTContext) (result *APIGoal, resultErr error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if target == nil {
		return nil, ErrSMTContextOwner
	}
	if err := target.open(); err != nil {
		return nil, err
	}
	if target.smtContextState == g.owner {
		return nil, ast.ErrASTDomain
	}
	targetManager, err := target.Manager()
	if err != nil {
		return nil, err
	}
	translation, err := ast.NewASTTranslation(g.manager, targetManager, true)
	if err != nil {
		return nil, err
	}
	translated, err := g.goal.Translate(translation)
	if err != nil {
		return nil, errors.Join(err, translation.Close())
	}
	if err = translation.Close(); err != nil {
		return nil, errors.Join(err, translated.Close())
	}
	result = &APIGoal{owner: target.smtContextState, manager: targetManager, goal: translated}
	result.self = result
	return result, nil
}
