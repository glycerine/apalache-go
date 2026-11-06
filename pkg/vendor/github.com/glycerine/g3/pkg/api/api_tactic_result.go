package api

// Source: src/api/api_tactic.cpp:461-531.

import (
	"errors"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type APITacticResult struct {
	self           *APITacticResult
	owner          *smtContextState
	manager        *ast.ASTManager
	goals          []*tactic.Goal
	goalCleanup    []bool
	proofConverter tactic.ProofConverter
	core           tactic.DependencyRef
	modelConverter tactic.ModelConverter
	modelCleanup   bool
	closed         bool
}

func (r *APITacticResult) open() error {
	if r == nil || r.self != r || r.owner == nil || r.manager == nil || r.closed || r.owner.closed || r.owner.command == nil {
		return ErrSMTContextOwner
	}
	return nil
}

func (r *APITacticResult) Close() (result error) {
	if r == nil || r.closed {
		return nil
	}
	if r.self != r {
		return ErrSMTContextOwner
	}
	if err := r.core.Close(); err != nil {
		return err
	}
	if r.proofConverter != nil {
		if err := r.proofConverter.DecRef(); err != nil {
			return err
		}
		r.proofConverter = nil
	}
	if r.modelConverter != nil {
		var err error
		if r.modelCleanup {
			err = r.modelConverter.Close()
		} else {
			var consumed bool
			consumed, err = r.modelConverter.DecRefConsumed()
			if consumed {
				r.modelCleanup = err != nil
			}
		}
		if err != nil {
			return err
		}
		r.modelConverter = nil
	}
	for len(r.goalCleanup) < len(r.goals) {
		r.goalCleanup = append(r.goalCleanup, false)
	}
	for i := len(r.goals) - 1; i >= 0; i-- {
		if r.goals[i] == nil {
			continue
		}
		var err error
		if r.goalCleanup[i] {
			err = r.goals[i].Close()
		} else {
			var consumed bool
			consumed, err = r.goals[i].DecRefConsumed()
			if consumed {
				r.goalCleanup[i] = err != nil
			}
		}
		if err != nil {
			result = errors.Join(result, err)
		} else {
			r.goals[i] = nil
		}
	}
	if result == nil {
		r.goals = nil
		r.goalCleanup = nil
		r.closed = true
		r.owner, r.manager = nil, nil
	}
	return result
}

func (r *APITacticResult) NumSubgoals() (uint32, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	return uint32(len(r.goals)), nil
}

// Subgoal owns one reference to the source shared goal. Mutations through
// another wrapper are visible, and the goal may outlive its apply result.
func (r *APITacticResult) Subgoal(index uint32) (*APIGoal, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if uint64(index) >= uint64(len(r.goals)) {
		return nil, ast.ErrASTDomain
	}
	goal := r.goals[index]
	if err := goal.IncRef(); err != nil {
		return nil, err
	}
	result := &APIGoal{owner: r.owner, manager: r.manager, goal: goal, shared: true}
	result.self = result
	return result, nil
}

func (r *APITacticResult) String() (string, error) {
	if err := r.open(); err != nil {
		return "", err
	}
	var output strings.Builder
	output.WriteString("(goals\n")
	for _, goal := range r.goals {
		text, err := apiGoalString(r.owner, r.manager, goal)
		if err != nil {
			return "", err
		}
		output.WriteString(text)
		output.WriteByte('\n')
	}
	output.WriteByte(')')
	return output.String(), nil
}
