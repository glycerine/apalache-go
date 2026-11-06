package qe

// Source: src/qe/qe.cpp, process_partition for scalar variables.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

// processExprQEScalarPartition eliminates each independent group in source
// reverse order, then installs and blocks the resulting unit branch.
func processExprQEScalarPartition(node *exprQETree, partition *exprQETreePartition, solver *smtcore.SMTSolver, config Config) (complete bool, err error) {
	if node == nil || node.manager == nil || partition == nil || partition.manager != node.manager || !partition.split || solver == nil {
		return false, ast.ErrASTHandle
	}
	m := node.manager
	original, err := m.RefGet(node.formula)
	if err != nil {
		return false, err
	}
	formula, err := m.NewRef(original)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&formula)) }()
	for {
		group, ok, e := partition.popVariables(node)
		if e != nil {
			return false, e
		}
		if !ok {
			break
		}
		current, e := m.RefGet(formula)
		if e != nil {
			return false, e
		}
		bound, e := bindExprQuantifierVariables(m, group, current)
		if e != nil {
			return false, e
		}
		boundValue, e := m.RefGet(bound)
		if e != nil {
			return false, errors.Join(e, m.DelRef(&bound))
		}
		kind, e := m.GetKind(boundValue)
		if e != nil {
			return false, errors.Join(e, m.DelRef(&bound))
		}
		if kind == ast.ASTQuantifier {
			eliminated, done, cause := rewriteQuantifierScalar(m, boundValue, config)
			if cause != nil || !done {
				return false, errors.Join(cause, m.DelRef(&bound))
			}
			h, cause := m.RefGet(eliminated)
			if cause == nil {
				cause = m.SetRef(formula, h)
			}
			cause = errors.Join(cause, m.DelRef(&eliminated), m.DelRef(&bound))
			if cause != nil {
				return false, cause
			}
		} else if e = m.DelRef(&bound); e != nil {
			return false, e
		}
	}
	value, err := m.RefGet(formula)
	if err != nil {
		return false, err
	}
	child, err := node.addUnit(config, value)
	if err != nil {
		return false, err
	}
	for len(child.variables) > 0 {
		i := len(child.variables) - 1
		if err = m.DelRef(&child.variables[i]); err != nil {
			return false, err
		}
		child.variables = child.variables[:i]
	}
	if err = assertExprQETreeConstraint(child, solver, true); err != nil {
		return false, err
	}
	return true, nil
}
