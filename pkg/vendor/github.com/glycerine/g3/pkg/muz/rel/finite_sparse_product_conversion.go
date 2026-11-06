package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// CanConvertToTableRelation reports whether this product has no inner columns,
// matching finite_product_relation_plugin::can_convert_to_table_relation.
func (p *FiniteSparseProduct) CanConvertToTableRelation() (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	return len(p.innerCols) == 0, nil
}

// NewFiniteSparseProductFromTableRelation embeds every table row under an
// empty inner relation, matching the source conversion for empty inner
// signatures.
func NewFiniteSparseProductFromTableRelation(source *TableRelation) (*FiniteSparseProduct, error) {
	if err := source.open(); err != nil {
		return nil, err
	}
	result, err := NewFiniteSparseProduct(source.manager, source.sorts, nil)
	if err != nil {
		return nil, err
	}
	facts, err := source.Facts()
	if err != nil {
		return nil, errors.Join(err, result.Close())
	}
	for _, fact := range facts {
		key, inner := result.splitEncoded(fact)
		if err = result.addEncodedFact(key, inner); err != nil {
			return nil, errors.Join(err, result.Close())
		}
	}
	return result, nil
}

// NewFiniteSparseProductFromInnerSparseTable wraps an inner relation under
// the source singleton outer key and owns a clone of the supplied relation.
func NewFiniteSparseProductFromInnerSparseTable(manager *RelationManager, sorts []ast.ASTHandle, source *SparseTable) (_ *FiniteSparseProduct, resultErr error) {
	if source == nil || source.open() != nil || source.functional != 0 || manager == nil || manager.open() != nil {
		return nil, ErrTableRelation
	}
	if len(sorts) != source.signature.Len() {
		return nil, ErrTableRelation
	}
	for column, sort := range sorts {
		domain, err := manager.sortDomain(sort)
		if err != nil {
			return nil, err
		}
		if source.signature.domains[column] != domain {
			return nil, ErrTableRelation
		}
	}
	tableColumns := make([]bool, len(sorts))
	result, err := NewFiniteSparseProduct(manager, sorts, tableColumns)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	inner, err := source.Clone()
	if err != nil {
		return nil, err
	}
	result.inner = append(result.inner, inner)
	if _, err = result.table.Add(Fact{0}); err != nil {
		return nil, err
	}
	keep = true
	return result, nil
}

// ToTableRelation converts products with an empty inner signature into a
// standalone table relation.
func (p *FiniteSparseProduct) ToTableRelation() (*TableRelation, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	canConvert, err := p.CanConvertToTableRelation()
	if err != nil {
		return nil, err
	}
	if !canConvert {
		return nil, ErrTableRelation
	}
	result, err := NewTableRelation(p.manager, p.sorts)
	if err != nil {
		return nil, err
	}
	flat, err := p.flatten()
	if err != nil {
		return nil, errors.Join(err, result.Close())
	}
	defer flat.Close()
	facts, err := flat.Facts()
	if err != nil {
		return nil, errors.Join(err, result.Close())
	}
	for _, fact := range facts {
		if _, err = result.AddTableFact(fact); err != nil {
			return nil, errors.Join(err, result.Close())
		}
	}
	return result, nil
}
