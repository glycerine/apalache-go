package rel

// Source: src/muz/rel/check_relation.cpp, checked filter operations.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func (r *relationFormulaChecker) verifyGrounded(left, right ast.ASTHandle, named bool) (resultErr error) {
	a, err := r.groundWithNames(left, named)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&a)) }()
	b, err := r.groundWithNames(right, named)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&b)) }()
	x, err := r.manager.RefGet(a)
	if err != nil {
		return err
	}
	y, err := r.manager.RefGet(b)
	if err != nil {
		return err
	}
	return r.checkEquivalent(x, y)
}

// verifyFilter checks old_formula AND condition against the current table and
// replaces the stored formula only after verification succeeds.
func (r *FormulaCheckedTable) verifyFilter(condition ast.ASTHandle) (resultErr error) {
	old, err := r.manager.RefGet(r.formula)
	if err != nil {
		return err
	}
	expected, err := r.manager.MkAndArgs([]ast.ASTHandle{old, condition})
	if err != nil {
		return err
	}
	expectedRef, err := r.manager.NewRef(expected)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&expectedRef)) }()
	actual, err := r.table.ToFormula()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&actual)) }()
	actualHandle, err := r.manager.RefGet(actual)
	if err != nil {
		return err
	}
	if err = r.verifyGrounded(expected, actualHandle, true); err != nil {
		return fmt.Errorf("filter: %w", err)
	}
	return r.manager.SetRef(r.formula, actualHandle)
}

// FilterEqual uses the source filter_equal verification and numeric grounding.
func (r *FormulaCheckedTable) FilterEqual(value ast.ASTHandle, column int) (resultErr error) {
	if err := r.open(); err != nil {
		return err
	}
	if column < 0 || column >= len(r.table.sorts) {
		return ErrTableRelation
	}
	variable, err := r.manager.MkVar(uint32(column), r.table.sorts[column])
	if err != nil {
		return err
	}
	varRef, err := r.manager.NewRef(variable)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&varRef)) }()
	condition, err := r.manager.MkEq(variable, value)
	if err != nil {
		return err
	}
	conditionRef, err := r.manager.NewRef(condition)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&conditionRef)) }()
	old, err := r.manager.RefGet(r.formula)
	if err != nil {
		return err
	}
	oldRef, err := r.manager.NewRef(old)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&oldRef)) }()
	if err = r.table.FilterEqual(value, column); err != nil {
		return err
	}
	actual, err := r.table.ToFormula()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&actual)) }()
	actualHandle, err := r.manager.RefGet(actual)
	if err != nil {
		return err
	}
	if err = r.manager.SetRef(r.formula, actualHandle); err != nil {
		return err
	}
	expected, err := r.manager.MkAndArgs([]ast.ASTHandle{old, condition})
	if err != nil {
		return err
	}
	expectedRef, err := r.manager.NewRef(expected)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&expectedRef)) }()
	if err = r.verifyGrounded(expected, actualHandle, false); err != nil {
		return fmt.Errorf("filter_equal: %w", err)
	}
	return nil
}

func (r *FormulaCheckedTable) FilterIdentical(columns []int) (resultErr error) {
	if err := r.open(); err != nil {
		return err
	}
	if len(columns) == 0 {
		return ErrTableRelation
	}
	for i, col := range columns {
		if col < 0 || col >= len(r.table.sorts) || i > 0 && r.table.sorts[col] != r.table.sorts[columns[0]] {
			return ast.ErrASTDomain
		}
	}
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, r.manager.DelRef(&refs[i]))
		}
	}()
	hold := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := r.manager.NewRef(value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		return value, nil
	}
	first, err := hold(r.manager.MkVar(uint32(columns[0]), r.table.sorts[columns[0]]))
	if err != nil {
		return err
	}
	var conditions []ast.ASTHandle
	for _, col := range columns[1:] {
		variable, err := hold(r.manager.MkVar(uint32(col), r.table.sorts[col]))
		if err != nil {
			return err
		}
		condition, err := hold(r.manager.MkEq(first, variable))
		if err != nil {
			return err
		}
		conditions = append(conditions, condition)
	}
	condition, err := hold(r.manager.MkAndArray(conditions))
	if err != nil {
		return err
	}
	if err = r.table.FilterIdentical(columns); err != nil {
		return err
	}
	return r.verifyFilter(condition)
}

func (r *FormulaCheckedTable) FilterInterpreted(theory *rewriter.TheoryRewriter, condition ast.ASTHandle) error {
	if err := r.open(); err != nil {
		return err
	}
	columns, sorts, err := r.interpretedVariables(condition)
	if err != nil {
		return err
	}
	if err = r.table.FilterInterpreted(theory, []ast.ASTHandle{condition}, columns, sorts); err != nil {
		return err
	}
	return r.verifyFilter(condition)
}

func (r *FormulaCheckedTable) interpretedVariables(condition ast.ASTHandle) (map[uint32]int, map[uint32]ast.ASTHandle, error) {
	used, err := ast.NewASTUsedVars(r.manager)
	if err != nil {
		return nil, nil, err
	}
	if err = used.Collect(condition); err != nil {
		return nil, nil, err
	}
	if uint64(used.MaxFoundVarIndexPlusOne()) > uint64(len(r.table.sorts)) {
		return nil, nil, ast.ErrASTDomain
	}
	columns, sorts := make(map[uint32]int), make(map[uint32]ast.ASTHandle)
	for i := uint32(0); i < used.MaxFoundVarIndexPlusOne(); i++ {
		if sort := used.Contains(i); sort != (ast.ASTHandle{}) {
			if sort != r.table.sorts[i] {
				return nil, nil, ast.ErrASTDomain
			}
			columns[i], sorts[i] = int(i), sort
		}
	}
	return columns, sorts, nil
}
