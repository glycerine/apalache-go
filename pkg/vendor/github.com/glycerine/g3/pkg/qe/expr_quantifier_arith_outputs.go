package qe

// Source: src/qe/qe.cpp, arithmetic solver callback outputs during branching.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

func drainExprQEArithmeticOutputs(plugin *ArithmeticPlugin, node *exprQETree, variables *exprQEBranchVariables, solver *smtcore.SMTSolver) (err error) {
	if plugin == nil || node == nil || node.manager == nil || variables == nil || solver == nil {
		return ast.ErrASTHandle
	}
	m := node.manager
	constraints, err := plugin.TakeConstraints()
	if err != nil {
		return err
	}
	if err = drainExprQEConstraints(constraints, node, solver); err != nil {
		return err
	}
	fresh, err := plugin.TakeFreshVariables()
	if err != nil {
		return err
	}
	defer func() {
		for i := range fresh {
			if fresh[i] != (ast.ASTRef{}) {
				err = errors.Join(err, m.DelRef(&fresh[i]))
			}
		}
	}()
	for i := range fresh {
		h, e := m.RefGet(fresh[i])
		if e != nil {
			return e
		}
		_, found, e := plugin.context.IsVariable(h)
		if e != nil {
			return e
		}
		if !found {
			if e = plugin.context.adoptVariable(h, &fresh[i]); e != nil {
				return e
			}
		}
		if e = variables.add(h); e != nil {
			return e
		}
	}
	return nil
}

func drainExprQEConstraints(constraints []ArithmeticConstraint, node *exprQETree, solver *smtcore.SMTSolver) (err error) {
	m := node.manager
	defer func() {
		for i := range constraints {
			if constraints[i].Formula != (ast.ASTRef{}) {
				err = errors.Join(err, m.DelRef(&constraints[i].Formula))
			}
		}
	}()
	for _, constraint := range constraints {
		h, e := m.RefGet(constraint.Formula)
		if e != nil {
			return e
		}
		if e = assertExprQETreeConstraint(node, solver, constraint.UseVariable, h); e != nil {
			return e
		}
	}
	return nil
}
