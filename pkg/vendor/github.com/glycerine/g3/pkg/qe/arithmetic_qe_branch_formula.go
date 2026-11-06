package qe

// Source: src/qe/qe_arith_plugin.cpp branch_formula payload ownership.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type ArithmeticBranchFormula struct {
	manager     *ast.ASTManager
	Key         ArithmeticBranchKey
	Formula     ast.ASTRef
	Variable    ast.ASTRef
	Result      ast.ASTRef
	Term        ast.ASTRef
	Coefficient *big.Rat
	Variables   []ast.ASTRef
}

func NewArithmeticBranchFormula(manager *ast.ASTManager, formula, variable ast.ASTHandle, branch uint32, result ast.ASTHandle, coefficient *big.Rat, term ast.ASTHandle, variables []ast.ASTHandle) (record *ArithmeticBranchFormula, resultErr error) {
	if manager == nil || coefficient == nil {
		return nil, ast.ErrASTHandle
	}
	record = &ArithmeticBranchFormula{manager: manager, Key: ArithmeticBranchKey{Formula: formula, Variable: variable, Branch: branch}, Coefficient: new(big.Rat).Set(coefficient)}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, record.Close())
			record = nil
		}
	}()
	var err error
	if record.Formula, err = manager.NewRef(formula); err != nil {
		return record, err
	}
	if record.Variable, err = manager.NewRef(variable); err != nil {
		return record, err
	}
	if record.Result, err = manager.NewRef(result); err != nil {
		return record, err
	}
	if !term.IsNull() {
		if record.Term, err = manager.NewRef(term); err != nil {
			return record, err
		}
	}
	for _, fresh := range variables {
		ref, cause := manager.NewRef(fresh)
		if cause != nil {
			return record, cause
		}
		record.Variables = append(record.Variables, ref)
	}
	return record, nil
}

func (r *ArithmeticBranchFormula) Close() (result error) {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	for i := len(r.Variables) - 1; i >= 0; i-- {
		result = errors.Join(result, r.manager.DelRef(&r.Variables[i]))
	}
	r.Variables = nil
	for _, ref := range []*ast.ASTRef{&r.Term, &r.Result, &r.Variable, &r.Formula} {
		if *ref != (ast.ASTRef{}) {
			result = errors.Join(result, r.manager.DelRef(ref))
		}
	}
	if result == nil {
		r.manager = nil
	}
	return result
}
