package qe

// Source: src/qe/qe.cpp, quant_elim_plugin::add_constraint.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

func assertExprQETreeConstraint(current *exprQETree, solver *smtcore.SMTSolver, useCurrent bool, literals ...ast.ASTHandle) (err error) {
	if current == nil || current.manager == nil || solver == nil || len(literals) > 3 {
		return ast.ErrASTDomain
	}
	m := current.manager
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			err = errors.Join(err, m.DelRef(&refs[i]))
		}
	}()
	var args []ast.ASTHandle
	if !useCurrent {
		current = current.parent
	}
	for node := current; node != nil; node = node.parent {
		assignment, e := m.RefGet(node.assignment)
		if e != nil {
			return e
		}
		negated, e := m.MkNotSimplified(assignment)
		if e != nil {
			return e
		}
		ref, e := m.NewRef(negated)
		if e != nil {
			return e
		}
		refs = append(refs, ref)
		args = append(args, negated)
	}
	for _, lit := range literals {
		if !lit.IsNull() {
			args = append(args, lit)
		}
	}
	clause, err := m.MkOrArgs(args)
	if err != nil {
		return err
	}
	ref, err := m.NewRef(clause)
	if err != nil {
		return err
	}
	refs = append(refs, ref)
	return solver.AssertFormula(clause)
}
