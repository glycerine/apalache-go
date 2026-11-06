package core

// Source: src/tactic/core/injectivity_tactic.cpp operator().

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func (t *InjectivityTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	t.running = true
	defer func() { t.running = false }()
	if sorted, e := goal.IsWellSorted(); e != nil {
		return e
	} else if !sorted {
		return ast.ErrASTDomain
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("injectivity", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	if err = t.discover(goal); err != nil {
		return err
	}
	if err = t.traversal.Reset(); err != nil {
		return err
	}
	out, err := t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&out)) }()
	n, err := goal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		formula, e := goal.Form(i)
		if e != nil {
			return e
		}
		if e = t.traversal.Rewrite(formula, out); e != nil {
			return e
		}
		rewritten, e := t.manager.RefGet(out)
		if e != nil {
			return e
		}
		dependency, e := goal.Dependency(i)
		if e != nil {
			return e
		}
		if e = goal.Update(i, rewritten, ast.ASTHandle{}, dependency); e != nil {
			return e
		}
	}
	return result.AddGoal(goal)
}
