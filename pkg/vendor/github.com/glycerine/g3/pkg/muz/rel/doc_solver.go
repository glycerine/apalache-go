package rel

// Source: src/muz/rel/doc.cpp is_empty_complete, to_formula, and check_equiv
// solver integration. Projection variable rewriting remains with its caller.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (m *DocManager) ToFormula(manager *ast.ASTManager, document *Doc) (ast.ASTHandle, error) {
	if manager == nil {
		return ast.ASTHandle{}, ErrDOC
	}
	if err := m.check(document); err != nil {
		return ast.ASTHandle{}, err
	}
	conjunction := make([]ast.ASTHandle, 0, 1+len(document.negative.values))
	positive, err := m.vectors.ToFormula(manager, document.positive)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if manager.IsFalse(positive) {
		return manager.MkFalse()
	}
	if !manager.IsTrue(positive) {
		conjunction = append(conjunction, positive)
	}
	for _, negative := range document.negative.values {
		formula, makeErr := m.vectors.ToFormula(manager, negative)
		if makeErr != nil {
			return ast.ASTHandle{}, makeErr
		}
		formula, makeErr = manager.MkNotSimplified(formula)
		if makeErr != nil {
			return ast.ASTHandle{}, makeErr
		}
		if manager.IsFalse(formula) {
			return manager.MkFalse()
		}
		if !manager.IsTrue(formula) {
			conjunction = append(conjunction, formula)
		}
	}
	return manager.MkAndArray(conjunction)
}

// IsEmptyComplete uses the retained legacy SMT kernel, matching the source
// completeness fallback instead of treating the structural containment test
// as exact.
func (m *DocManager) IsEmptyComplete(manager *ast.ASTManager, document *Doc) (_ bool, resultErr error) {
	if err := m.check(document); err != nil {
		return false, err
	}
	if len(document.negative.values) == 0 {
		return false, nil
	}
	formula, err := m.ToFormula(manager, document)
	if err != nil {
		return false, err
	}
	kernel, err := smtcore.NewSMTKernel(manager, nil)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, kernel.Close()) }()
	if err = kernel.Assert(formula, ast.ASTHandle{}); err != nil {
		return false, err
	}
	status, err := kernel.Check()
	if err != nil {
		return false, err
	}
	if status == lbool.True {
		return false, nil
	}
	// Source native code returns empty after its debug-only l_false assertion.
	return true, nil
}

func (u *DocUnion) EmptyComplete(manager *ast.ASTManager) (bool, error) {
	if err := u.check(); err != nil {
		return false, err
	}
	for _, document := range u.values {
		empty, err := u.owner.IsEmptyComplete(manager, document)
		if err != nil {
			return false, err
		}
		if !empty {
			return false, nil
		}
	}
	return true, nil
}
