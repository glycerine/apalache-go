package rel

// Source: src/muz/rel/dl_product_relation.cpp join_fn T_FULL component
// completion and product_relation_plugin::mk_full/mk_empty.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// NewSparseIntervalSieveProductAuxiliary constructs evaluator-generated
// intermediates. Unlike the caller-visible representation selector, auxiliary
// signatures may be purely finite, purely arithmetic, or nullary.
func NewSparseIntervalSieveProductAuxiliary(manager *RelationManager, sorts []ast.ASTHandle, full bool) (*SparseIntervalSieveProduct, error) {
	return newSparseIntervalSieveProduct(manager, sorts, full, false)
}

// AppendFullColumn returns an independent product with one unconstrained
// column appended, matching the source join with a T_FULL component.
func (p *SparseIntervalSieveProduct) AppendFullColumn(sort ast.ASTHandle) (*SparseIntervalSieveProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	full, err := NewSparseIntervalSieveProductAuxiliary(p.manager, []ast.ASTHandle{sort}, true)
	if err != nil {
		return nil, err
	}
	result, joinErr := JoinSparseIntervalSieveProducts(p, full, nil, nil)
	closeErr := full.Close()
	if joinErr != nil {
		return nil, errors.Join(joinErr, closeErr)
	}
	if closeErr != nil {
		return nil, errors.Join(closeErr, result.Close())
	}
	return result, nil
}

// DuplicateColumn appends a column constrained equal to source. The receiver
// remains unchanged.
func (p *SparseIntervalSieveProduct) DuplicateColumn(source int) (*SparseIntervalSieveProduct, error) {
	if err := p.open(); err != nil || source < 0 || source >= len(p.sorts) {
		if err != nil {
			return nil, err
		}
		return nil, ErrSparseIntervalProduct
	}
	result, err := p.AppendFullColumn(p.sorts[source])
	if err != nil {
		return nil, err
	}
	if err = result.FilterIdentical([]int{source, len(p.sorts)}); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	return result, nil
}
