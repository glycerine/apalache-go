package api

// Source: src/api/api_goal.cpp:75-82,100-106.

import "github.com/glycerine/g3/pkg/ast"

func (g *APIGoal) Assert(formula ast.ASTHandle) error {
	if err := g.open(); err != nil {
		return err
	}
	sort, err := g.manager.GetSort(formula)
	if err != nil {
		return err
	}
	boolean, err := g.manager.MkBoolSort()
	if err != nil {
		return err
	}
	if sort != boolean {
		return ast.ErrASTDomain
	}
	return g.goal.Assert(formula)
}

func (g *APIGoal) Reset() error {
	if err := g.open(); err != nil {
		return err
	}
	return g.goal.Reset()
}
