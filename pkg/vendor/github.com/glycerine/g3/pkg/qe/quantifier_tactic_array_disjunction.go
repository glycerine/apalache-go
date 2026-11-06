package qe

// Source: src/qe/qe.cpp, quantifier elimination over Boolean branches.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// projectArrayDisjunction distributes projection over an OR reached solely
// through AND nodes; those positions are positive in the input formula.
func projectArrayDisjunction(m *ast.ASTManager, formula ast.ASTHandle, context *SolverContext, config Config) (result ast.ASTRef, complete bool, err error) {
	defer func() {
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
		}
	}()
	todo := []ast.ASTHandle{formula}
	var disjunction ast.ASTHandle
	for len(todo) > 0 {
		last := len(todo) - 1
		current := todo[last]
		todo = todo[:last]
		kind, e := m.GetKind(current)
		if e != nil {
			return result, false, e
		}
		if kind != ast.ASTApp {
			continue
		}
		family, e := m.GetFamilyID(current)
		if e != nil {
			return result, false, e
		}
		if family != m.BasicFamilyID() {
			continue
		}
		op, e := m.GetDeclKind(current)
		if e != nil {
			return result, false, e
		}
		if op == ast.ASTOpOr {
			disjunction = current
			break
		}
		if op != ast.ASTOpAnd {
			continue
		}
		n, e := m.GetNumArgs(current)
		if e != nil {
			return result, false, e
		}
		for i := n; i > 0; i-- {
			arg, cause := m.GetArg(current, i-1)
			if cause != nil {
				return result, false, cause
			}
			todo = append(todo, arg)
		}
	}
	if disjunction.IsNull() {
		return result, false, nil
	}
	count, err := m.GetNumArgs(disjunction)
	if err != nil {
		return result, false, err
	}
	replace, err := rewriter.NewExprSafeReplace(m)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, replace.Close()) }()
	var projected []ast.ASTRef
	defer func() {
		for i := len(projected) - 1; i >= 0; i-- {
			err = errors.Join(err, m.DelRef(&projected[i]))
		}
	}()
	for i := uint32(0); i < count; i++ {
		alternative, e := m.GetArg(disjunction, i)
		if e != nil {
			return result, false, e
		}
		branch, e := m.NewRef(formula)
		if e != nil {
			return result, false, e
		}
		if e = replace.ApplySubstitution(disjunction, alternative, branch); e != nil {
			return result, false, errors.Join(e, m.DelRef(&branch))
		}
		value, e := m.RefGet(branch)
		if e != nil {
			return result, false, errors.Join(e, m.DelRef(&branch))
		}
		bound, e := bindExprQuantifierVariables(m, context.variables, value)
		e = errors.Join(e, m.DelRef(&branch))
		if e != nil {
			return result, false, e
		}
		value, e = m.RefGet(bound)
		if e != nil {
			return result, false, errors.Join(e, m.DelRef(&bound))
		}
		kind, e := m.GetKind(value)
		if e != nil {
			return result, false, errors.Join(e, m.DelRef(&bound))
		}
		if kind != ast.ASTQuantifier {
			projected = append(projected, bound)
			continue
		}
		part, done, e := eliminateQuantifierArrayEquations(m, value, config)
		e = errors.Join(e, m.DelRef(&bound))
		if e != nil || !done {
			return result, false, e
		}
		projected = append(projected, part)
	}
	values := make([]ast.ASTHandle, len(projected))
	for i, ref := range projected {
		values[i], err = m.RefGet(ref)
		if err != nil {
			return result, false, err
		}
	}
	joined, err := m.MkOrArgs(values)
	if err != nil {
		return result, false, err
	}
	result, err = m.NewRef(joined)
	return result, err == nil, err
}
