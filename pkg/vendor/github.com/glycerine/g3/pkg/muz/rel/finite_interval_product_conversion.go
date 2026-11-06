package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp, conversions between
// finite products and the table or inner relation plugins.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// FiniteIntervalProductFromTable copies a table relation into a finite product
// whose rows share one full zero-column interval relation.
func FiniteIntervalProductFromTable(source *TableRelation) (_ *FiniteIntervalProduct, resultErr error) {
	if source == nil || source.open() != nil {
		return nil, ErrTableRelation
	}
	mask := make([]bool, len(source.sorts))
	for i := range mask {
		mask[i] = true
	}
	result, err := NewFiniteIntervalProduct(source.manager, source.sorts, mask)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	result.intervals = append(result.intervals, NewIntervalFull(0))
	rows, err := source.table.Facts()
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		row = append(row, 0)
		if err = result.table.EnsureFact(row); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// FiniteIntervalProductFromInterval copies an interval relation into a finite
// product with no table data columns and one functional row.
func FiniteIntervalProductFromInterval(manager *RelationManager, sorts []ast.ASTHandle, source *IntervalRelation) (_ *FiniteIntervalProduct, resultErr error) {
	if source == nil || source.values == nil || len(source.values.columns) != len(sorts) {
		return nil, ast.ErrASTDomain
	}
	result, err := NewFiniteIntervalProduct(manager, sorts, make([]bool, len(sorts)))
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	if err = result.storeInner(Fact{0}, cloneIntervalRelation(source)); err != nil {
		return nil, err
	}
	return result, nil
}

// ToTableRelation converts a product with no interval columns into an owned
// table relation after removing rows with empty inner relations.
func (p *FiniteIntervalProduct) ToTableRelation() (_ *TableRelation, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if len(p.intervalColumns) != 0 {
		return nil, ErrTableRelation
	}
	if err := p.garbageCollect(true); err != nil {
		return nil, err
	}
	result, err := NewTableRelation(p.manager, p.sorts)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	rows, err := p.table.Facts()
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if _, err = result.table.Add(row[:len(row)-1]); err != nil {
			return nil, err
		}
	}
	return result, nil
}
