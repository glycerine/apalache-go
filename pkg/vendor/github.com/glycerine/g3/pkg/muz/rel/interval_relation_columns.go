package rel

// Source: dl_compiler.cpp make_add_unbound_column,
// make_add_constant_column, and make_duplicate_column for the concrete
// interval_relation instantiation.

import "github.com/glycerine/g3/pkg/ast"

func (r *IntervalRelation) validateColumnSignature(manager *ast.ASTManager, sorts []ast.ASTHandle) error {
	if r == nil || r.values == nil || manager == nil || len(sorts) != len(r.values.columns) || !CanHandleIntervalSorts(manager, sorts) {
		return ast.ErrASTDomain
	}
	return nil
}

// AppendFullColumn returns an independent relation with one unconstrained
// arithmetic column appended. It is the concrete interval lowering of the
// source compiler's cached total unary relation join.
func (r *IntervalRelation) AppendFullColumn(manager *ast.ASTManager, sorts []ast.ASTHandle, sort ast.ASTHandle) (*IntervalRelation, error) {
	if err := r.validateColumnSignature(manager, sorts); err != nil {
		return nil, err
	}
	if !CanHandleIntervalSorts(manager, []ast.ASTHandle{sort}) {
		return nil, ast.ErrASTDomain
	}
	return JoinIntervalRelations(r, NewIntervalFull(1), nil, nil)
}

// AppendConstantColumn appends a unary arithmetic singleton without keeping
// the source AST alive after its numeral has been copied into the interval.
func (r *IntervalRelation) AppendConstantColumn(manager *ast.ASTManager, sorts []ast.ASTHandle, sort, value ast.ASTHandle) (*IntervalRelation, error) {
	if manager == nil {
		return nil, ast.ErrASTDomain
	}
	actual, err := manager.GetSort(value)
	if err != nil {
		return nil, err
	}
	if actual != sort {
		return nil, ast.ErrASTDomain
	}
	number, err := intervalEqualityValue(manager, value)
	if err != nil {
		return nil, err
	}
	result, err := r.AppendFullColumn(manager, sorts, sort)
	if err != nil {
		return nil, err
	}
	if err = result.FilterEqual(uint32(len(sorts)), number); err != nil {
		return nil, err
	}
	return result, nil
}

// DuplicateColumn appends a column constrained equal to an existing column,
// matching make_duplicate_column's project-and-equality-join result.
func (r *IntervalRelation) DuplicateColumn(manager *ast.ASTManager, sorts []ast.ASTHandle, column uint32) (*IntervalRelation, error) {
	if uint64(column) >= uint64(len(sorts)) {
		return nil, ast.ErrASTDomain
	}
	result, err := r.AppendFullColumn(manager, sorts, sorts[column])
	if err != nil {
		return nil, err
	}
	if err = result.FilterIdentical(column, uint32(len(sorts))); err != nil {
		return nil, err
	}
	return result, nil
}
