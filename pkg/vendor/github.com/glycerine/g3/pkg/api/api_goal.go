package api

// Source: src/api/api_goal.cpp:27-57.
//
// APIGoal is the public Go ownership bridge for Z3_goal_ref. It owns one
// concrete tactic goal and borrows its SMTContext; Close replaces opaque
// inc_ref/dec_ref traffic at the Go boundary.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type APIGoal struct {
	self            *APIGoal
	owner           *smtContextState
	manager         *ast.ASTManager
	goal            *tactic.Goal
	closed          bool
	shared, cleanup bool
}

func (c *SMTContext) NewGoal(models, unsatCores, proofs bool) (*APIGoal, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	goal, err := tactic.NewGoal(manager, proofs, models, unsatCores)
	if err != nil {
		return nil, err
	}
	result := &APIGoal{owner: c.smtContextState, manager: manager, goal: goal}
	result.self = result
	return result, nil
}

func (g *APIGoal) open() error {
	if g == nil || g.self != g || g.owner == nil || g.manager == nil || g.goal == nil || g.closed || g.owner.closed || g.owner.command == nil {
		return ErrSMTContextOwner
	}
	return g.goal.CheckOwner()
}

func (g *APIGoal) Close() error {
	if g == nil || g.closed {
		return nil
	}
	if g.self != g || g.goal == nil {
		return ErrSMTContextOwner
	}
	var err error
	if g.shared && !g.cleanup {
		var consumed bool
		consumed, err = g.goal.DecRefConsumed()
		if consumed {
			g.cleanup = err != nil
		}
	} else {
		err = g.goal.Close()
	}
	if err == nil {
		g.closed = true
		g.owner, g.manager, g.goal = nil, nil, nil
	}
	return err
}
