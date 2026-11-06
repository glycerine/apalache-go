package rel

// Source: src/muz/rel/dl_interval_relation.cpp
// interval_relation_plugin::can_handle_signature and filter_identical_fn.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// CanHandleIntervalSorts reports whether every sort is an integer or real
// arithmetic sort, matching interval_relation_plugin::can_handle_signature.
func CanHandleIntervalSorts(manager *ast.ASTManager, sorts []ast.ASTHandle) bool {
	if manager == nil {
		return false
	}
	family, err := manager.RegisterArithmeticPlugin()
	if err != nil {
		return false
	}
	for _, sort := range sorts {
		kind, err := manager.GetKind(sort)
		if err != nil || kind != ast.ASTSort {
			return false
		}
		actualFamily, err := manager.GetFamilyID(sort)
		if err != nil || actualFamily != family {
			return false
		}
		declKind, err := manager.GetDeclKind(sort)
		if err != nil || declKind < 0 || declKind > 1 {
			return false
		}
	}
	return true
}

// FilterIdenticalColumns intersects every listed column with the first one.
// Zero and one-column filters are source no-ops.
func (r *IntervalRelation) FilterIdenticalColumns(columns []uint32) error {
	if r == nil || r.values == nil {
		return ast.ErrASTDomain
	}
	if len(columns) < 2 {
		return nil
	}
	for _, column := range columns {
		if uint64(column) >= uint64(len(r.values.columns)) {
			return ast.ErrASTDomain
		}
	}
	for _, column := range columns[1:] {
		if err := r.values.equate(columns[0], column); err != nil {
			return err
		}
	}
	return nil
}

func intervalEqualityValue(manager *ast.ASTManager, value ast.ASTHandle) (*big.Rat, error) {
	if manager == nil {
		return nil, ast.ErrASTDomain
	}
	text, _, matched, err := manager.ArithmeticNumeralText(value)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, ast.ErrASTDomain
	}
	number, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, ast.ErrASTDomain
	}
	return number, nil
}

// FilterEqualExpression captures the arithmetic numeral from the source AST
// before intersecting the selected column, as filter_equal_fn does.
func (r *IntervalRelation) FilterEqualExpression(manager *ast.ASTManager, column uint32, value ast.ASTHandle) error {
	if r == nil || r.values == nil {
		return ast.ErrASTDomain
	}
	number, err := intervalEqualityValue(manager, value)
	if err != nil {
		return err
	}
	return r.FilterEqual(column, number)
}
