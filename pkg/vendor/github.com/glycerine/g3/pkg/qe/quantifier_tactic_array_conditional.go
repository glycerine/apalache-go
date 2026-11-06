package qe

// Source: src/qe/qe.cpp lift_ite and quantifier elimination branching.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// projectArrayConditional distributes an existential array projection over a
// relevant ITE and projects each guarded branch independently.
func projectArrayConditional(m *ast.ASTManager, formula ast.ASTHandle, context *SolverContext, config Config) (result ast.ASTRef, complete bool, err error) {
	defer func() {
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
		}
	}()
	adapter, err := NewSolverNNFAdapter(context)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, adapter.Close()) }()
	nnf, err := NewQENNFNormalizer(m, adapter, adapter, config)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, nnf.Close()) }()
	ite, found, err := nnf.findRelevantITE(formula)
	if err != nil || !found {
		return result, false, err
	}
	condition, err := m.GetArg(ite, 0)
	if err != nil {
		return result, false, err
	}
	replace, err := rewriter.NewExprSafeReplace(m)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, replace.Close()) }()
	var projected [2]ast.ASTRef
	defer func() {
		for i := range projected {
			if projected[i] != (ast.ASTRef{}) {
				err = errors.Join(err, m.DelRef(&projected[i]))
			}
		}
	}()
	for i := range projected {
		branchValue, e := m.GetArg(ite, uint32(i+1))
		if e != nil {
			return result, false, e
		}
		branch, e := m.NewRef(formula)
		if e != nil {
			return result, false, e
		}
		if e = replace.ApplySubstitution(ite, branchValue, branch); e != nil {
			return result, false, errors.Join(e, m.DelRef(&branch))
		}
		value, e := m.RefGet(branch)
		if e != nil {
			return result, false, errors.Join(e, m.DelRef(&branch))
		}
		guard := condition
		if i == 1 {
			guard, e = m.MkNot(condition)
			if e != nil {
				return result, false, errors.Join(e, m.DelRef(&branch))
			}
		}
		guarded, e := m.MkAnd(guard, value)
		if e != nil {
			return result, false, errors.Join(e, m.DelRef(&branch))
		}
		bound, e := bindExprQuantifierVariables(m, context.variables, guarded)
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
		if kind == ast.ASTQuantifier {
			var done bool
			projected[i], done, e = eliminateQuantifierArrayEquations(m, value, config)
			e = errors.Join(e, m.DelRef(&bound))
			if e != nil || !done {
				return result, false, e
			}
		} else {
			projected[i] = bound
		}
	}
	left, err := m.RefGet(projected[0])
	if err != nil {
		return result, false, err
	}
	right, err := m.RefGet(projected[1])
	if err != nil {
		return result, false, err
	}
	combined, err := m.MkOr(left, right)
	if err != nil {
		return result, false, err
	}
	result, err = m.NewRef(combined)
	return result, err == nil, err
}
