package rel

// Source: src/muz/rel/check_relation.cpp, mk_join and verify_join.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

func (r *FormulaCheckedTable) joinFormula(other *FormulaCheckedTable, columns, otherColumns []int) (result ast.ASTRef, resultErr error) {
	m := r.manager
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	hold := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := m.NewRef(value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		return value, nil
	}
	first, err := r.table.ToFormula()
	if err != nil {
		return result, err
	}
	refs = append(refs, first)
	second, err := other.table.ToFormula()
	if err != nil {
		return result, err
	}
	refs = append(refs, second)
	firstHandle, err := m.RefGet(first)
	if err != nil {
		return result, err
	}
	secondHandle, err := m.RefGet(second)
	if err != nil {
		return result, err
	}
	bindings := make([]ast.ASTHandle, len(other.table.sorts))
	for i, sort := range other.table.sorts {
		bindings[i], err = hold(m.MkVar(uint32(i+len(r.table.sorts)), sort))
		if err != nil {
			return result, err
		}
	}
	shifted, err := r.substitute(secondHandle, bindings)
	if err != nil {
		return result, err
	}
	refs = append(refs, shifted)
	shiftedHandle, err := m.RefGet(shifted)
	if err != nil {
		return result, err
	}
	formula, err := hold(m.MkAndArgs([]ast.ASTHandle{firstHandle, shiftedHandle}))
	if err != nil {
		return result, err
	}
	for i, column := range columns {
		left, err := hold(m.MkVar(uint32(column), r.table.sorts[column]))
		if err != nil {
			return result, err
		}
		rightColumn := otherColumns[i]
		right, err := hold(m.MkVar(uint32(rightColumn+len(r.table.sorts)), other.table.sorts[rightColumn]))
		if err != nil {
			return result, err
		}
		equality, err := hold(m.MkEq(left, right))
		if err != nil {
			return result, err
		}
		formula, err = hold(m.MkAndArgs([]ast.ASTHandle{equality, formula}))
		if err != nil {
			return result, err
		}
	}
	return m.NewRef(formula)
}

// Join checks the result of a concrete table join against the source formula
// conjunction and column equalities.
func (r *FormulaCheckedTable) Join(other *FormulaCheckedTable, columns, otherColumns []int) (_ *FormulaCheckedTable, resultErr error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if other.open() != nil || r.manager != other.manager || len(columns) != len(otherColumns) {
		return nil, ErrTableRelation
	}
	for i, column := range columns {
		oc := otherColumns[i]
		if column < 0 || column >= len(r.table.sorts) || oc < 0 || oc >= len(other.table.sorts) || r.table.sorts[column] != other.table.sorts[oc] {
			return nil, ast.ErrASTDomain
		}
	}
	table, err := r.table.Join(other.table, columns, otherColumns)
	if err != nil {
		return nil, err
	}
	result, err := NewFormulaCheckedTable(table, r.config)
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	formula, err := r.joinFormula(other, columns, otherColumns)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&formula)) }()
	expected, err := r.manager.RefGet(formula)
	if err != nil {
		return nil, err
	}
	actual, err := r.manager.RefGet(result.formula)
	if err != nil {
		return nil, err
	}
	if err = result.verifyGrounded(expected, actual, false); err != nil {
		return nil, fmt.Errorf("join: %w", err)
	}
	return result, nil
}

// JoinProject verifies the combined join and existential projection operation.
func (r *FormulaCheckedTable) JoinProject(other *FormulaCheckedTable, columns, otherColumns, removed []int) (_ *FormulaCheckedTable, resultErr error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if other.open() != nil || r.manager != other.manager || len(columns) != len(otherColumns) {
		return nil, ErrTableRelation
	}
	for i, column := range columns {
		oc := otherColumns[i]
		if column < 0 || column >= len(r.table.sorts) || oc < 0 || oc >= len(other.table.sorts) || r.table.sorts[column] != other.table.sorts[oc] {
			return nil, ast.ErrASTDomain
		}
	}
	if len(removed) == 0 {
		return nil, ast.ErrASTDomain
	}
	sorts := append(append([]ast.ASTHandle(nil), r.table.sorts...), other.table.sorts...)
	if _, err := tableRelationRemovedColumns(len(sorts), removed); err != nil {
		return nil, err
	}
	table, err := r.table.JoinProject(other.table, columns, otherColumns, removed)
	if err != nil {
		return nil, err
	}
	result, err := NewFormulaCheckedTable(table, r.config)
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	joined, err := r.joinFormula(other, columns, otherColumns)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&joined)) }()
	joinedHandle, err := r.manager.RefGet(joined)
	if err != nil {
		return nil, err
	}
	projected, err := r.projectFormulaForSorts(joinedHandle, sorts, removed)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&projected)) }()
	expected, err := r.manager.RefGet(projected)
	if err != nil {
		return nil, err
	}
	actual, err := r.manager.RefGet(result.formula)
	if err != nil {
		return nil, err
	}
	if err = result.verifyGrounded(expected, actual, false); err != nil {
		return nil, fmt.Errorf("join_project: %w", err)
	}
	return result, nil
}
