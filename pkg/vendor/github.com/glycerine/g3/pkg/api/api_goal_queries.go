package api

// Source: src/api/api_goal.cpp:59-73,84-98,108-152.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type APIGoalPrecision uint8

const (
	APIGoalPrecise APIGoalPrecision = iota
	APIGoalUnder
	APIGoalOver
	APIGoalUnderOver
)

func apiGoalPrecision(value tactic.GoalPrecision) (APIGoalPrecision, error) {
	switch value {
	case tactic.GoalPrecise:
		return APIGoalPrecise, nil
	case tactic.GoalUnder:
		return APIGoalUnder, nil
	case tactic.GoalOver:
		return APIGoalOver, nil
	case tactic.GoalUnderOver:
		return APIGoalUnderOver, nil
	default:
		return APIGoalUnderOver, ast.ErrASTDomain
	}
}

func (g *APIGoal) Precision() (APIGoalPrecision, error) {
	if err := g.open(); err != nil {
		return APIGoalUnderOver, err
	}
	value, err := g.goal.Precision()
	if err != nil {
		return APIGoalUnderOver, err
	}
	return apiGoalPrecision(value)
}

func (g *APIGoal) Inconsistent() (bool, error) {
	if err := g.open(); err != nil {
		return false, err
	}
	return g.goal.Inconsistent()
}

func (g *APIGoal) Depth() (uint32, error) {
	if err := g.open(); err != nil {
		return 0, err
	}
	return g.goal.Depth()
}

func (g *APIGoal) Size() (uint32, error) {
	if err := g.open(); err != nil {
		return 0, err
	}
	return g.goal.Size()
}

// Formula returns an owning reference, matching the native context trail that
// keeps the selected formula alive independently of later goal mutation.
func (g *APIGoal) Formula(index uint32) (ast.ASTRef, error) {
	if err := g.open(); err != nil {
		return ast.ASTRef{}, err
	}
	formula, err := g.goal.Form(index)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return g.manager.NewRef(formula)
}

func (g *APIGoal) NumExpressions() (uint32, error) {
	if err := g.open(); err != nil {
		return 0, err
	}
	return g.goal.NumExprs()
}

func (g *APIGoal) IsDecidedSat() (bool, error) {
	if err := g.open(); err != nil {
		return false, err
	}
	return g.goal.IsDecidedSat(), nil
}

func (g *APIGoal) IsDecidedUnsat() (bool, error) {
	if err := g.open(); err != nil {
		return false, err
	}
	return g.goal.IsDecidedUnsat(), nil
}
