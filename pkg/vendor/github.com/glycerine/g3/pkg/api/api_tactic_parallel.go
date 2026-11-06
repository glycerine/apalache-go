package api

// Source: src/api/api_tactic.cpp:127-148; tactical.cpp _NO_OMP_ branches.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

// ParallelOr uses the ordered source fallback for this single-goroutine port.
func (c *SMTContext) ParallelOr(children ...*APITactic) (*APITactic, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if len(children) == 0 {
		return nil, ast.ErrASTDomain
	}
	values := make([]tactic.Tactic, len(children))
	for i, child := range children {
		if err := child.validateOwner(c.smtContextState); err != nil {
			return nil, err
		}
		values[i] = child.tactic
	}
	value, err := tactic.NewParTactic(values...)
	if err != nil {
		return nil, err
	}
	return newAPITactic(c.smtContextState, value)
}

func (first *APITactic) ParallelAndThen(second *APITactic) (*APITactic, error) {
	owner, err := apiTacticPair(first, second)
	if err != nil {
		return nil, err
	}
	value, err := tactic.NewParAndThenTactic(first.tactic, second.tactic)
	if err != nil {
		return nil, err
	}
	return newAPITactic(owner, value)
}
