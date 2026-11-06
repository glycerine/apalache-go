package qe

// Source: src/qe/qe_sat_tactic.cpp, solver_context add_var/add_constraint.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *satSolverContext) takePartialArithmeticOutputs(plugin *ArithmeticPlugin) (err error) {
	if c == nil || c.closed || plugin == nil {
		return ast.ErrASTHandle
	}
	m := c.manager
	constraints, err := plugin.TakeConstraints()
	if err != nil {
		return err
	}
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
		if e = c.addConstraint(constraint.UseVariable, h); e != nil {
			return e
		}
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
	for _, ref := range fresh {
		h, e := m.RefGet(ref)
		if e != nil {
			return e
		}
		if e = c.addVariable(h); e != nil {
			return e
		}
	}
	return nil
}
