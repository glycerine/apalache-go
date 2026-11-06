package rel

// Source: dl_finite_product_relation.cpp converting_join_fn (313-356),
// specialized to the explanation plugin registered by dl_mk_explanations.cpp.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

// FiniteExplanationProductFromTable copies finite rows with a full
// zero-column explanation relation, as mk_from_table_relation does.
func FiniteExplanationProductFromTable(source *TableRelation) (result *FiniteExplanationProduct, resultErr error) {
	if source == nil || source.open() != nil {
		return nil, ErrTableRelation
	}
	return finiteExplanationProductFromHashTable(source.manager, source.sorts, source.table)
}

// The rule context stores concrete HashTables directly, so fact translation
// reaches the same source conversion without inventing an owning table wrapper.
func finiteExplanationProductFromHashTable(manager *RelationManager, sorts []ast.ASTHandle, table *HashTable) (result *FiniteExplanationProduct, resultErr error) {
	if table == nil {
		return nil, ErrTableRelation
	}
	return finiteExplanationProductFromFiniteTable(manager, sorts, table)
}

// finiteExplanationProductFromFiniteTable copies from the concrete finite
// representation selected for a predicate without replacing or aliasing it.
func finiteExplanationProductFromFiniteTable(manager *RelationManager, sorts []ast.ASTHandle, table FiniteTable) (result *FiniteExplanationProduct, resultErr error) {
	if table == nil {
		return nil, ErrTableRelation
	}
	expected, err := tableRelationSignature(manager, sorts)
	if err != nil {
		return nil, err
	}
	actual, err := table.Signature()
	if err != nil {
		return nil, err
	}
	if !expected.equal(actual) {
		return nil, ErrTableRelation
	}
	result, resultErr = newFiniteExplanationProduct(manager, sorts, 0)
	if resultErr != nil {
		return nil, resultErr
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	rows, err := table.Facts()
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		key := append(append(Fact(nil), row...), 0)
		if err = result.addEncodedRow(key, nil); err != nil {
			return result, err
		}
	}
	return result, nil
}

// FiniteExplanationProductFromExplanation copies an explanation row with no
// finite key columns, as mk_from_inner_relation does.
func FiniteExplanationProductFromExplanation(manager *RelationManager, source *ExplanationRelation) (result *FiniteExplanationProduct, resultErr error) {
	if source == nil || source.open() != nil || manager == nil || manager.open() != nil || source.manager != manager.manager {
		return nil, ErrTableRelation
	}
	result, resultErr = newFiniteExplanationProduct(manager, nil, len(source.sorts))
	if resultErr != nil {
		return nil, resultErr
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	if source.empty {
		return result, nil
	}
	row, err := source.Data()
	if err != nil {
		return result, err
	}
	if err = result.addEncodedRow(Fact{0}, row); err != nil {
		return result, err
	}
	return result, nil
}

func joinConvertedExplanationProduct(left, right, temporary *FiniteExplanationProduct, leftColumns, rightColumns []int) (result *FiniteExplanationProduct, resultErr error) {
	defer func() {
		resultErr = errors.Join(resultErr, temporary.Close())
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	return left.Join(right, leftColumns, rightColumns)
}

func (p *FiniteExplanationProduct) JoinTableRelation(right *TableRelation, leftColumns, rightColumns []int) (*FiniteExplanationProduct, error) {
	converted, err := FiniteExplanationProductFromTable(right)
	if err != nil {
		return nil, err
	}
	return joinConvertedExplanationProduct(p, converted, converted, leftColumns, rightColumns)
}
func (p *FiniteExplanationProduct) JoinFromTableRelation(left *TableRelation, leftColumns, rightColumns []int) (*FiniteExplanationProduct, error) {
	converted, err := FiniteExplanationProductFromTable(left)
	if err != nil {
		return nil, err
	}
	return joinConvertedExplanationProduct(converted, p, converted, leftColumns, rightColumns)
}
func (p *FiniteExplanationProduct) JoinExplanationRelation(right *ExplanationRelation) (*FiniteExplanationProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	converted, err := FiniteExplanationProductFromExplanation(p.manager, right)
	if err != nil {
		return nil, err
	}
	return joinConvertedExplanationProduct(p, converted, converted, nil, nil)
}
func (p *FiniteExplanationProduct) JoinFromExplanationRelation(left *ExplanationRelation) (*FiniteExplanationProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	converted, err := FiniteExplanationProductFromExplanation(p.manager, left)
	if err != nil {
		return nil, err
	}
	return joinConvertedExplanationProduct(converted, p, converted, nil, nil)
}
