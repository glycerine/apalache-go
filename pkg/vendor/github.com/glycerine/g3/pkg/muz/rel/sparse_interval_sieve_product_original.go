package rel

// Source: src/muz/rel/dl_product_relation.cpp join_fn::init T_FULL completion
// for concrete table-only and interval-only input relations.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// NewSparseIntervalFiniteOriginal constructs the concrete evaluator owner for
// a bounded finite original predicate. Its interval component is a full,
// transparent zero-column sieve.
func NewSparseIntervalFiniteOriginal(manager *RelationManager, sorts []ast.ASTHandle, full bool) (*SparseIntervalSieveProduct, error) {
	finiteColumns, _, _, err := sparseIntervalProductColumns(manager, sorts)
	if err != nil {
		return nil, err
	}
	if len(sorts) == 0 || len(finiteColumns) != len(sorts) {
		return nil, ErrSparseIntervalProduct
	}
	return newSparseIntervalSieveProduct(manager, sorts, full, false)
}

// NewSparseIntervalHashOriginal constructs the product used for an exact
// hashtable original. The evaluator's finite component is private; callers
// recover an owned HashTable through HashSnapshot, including for nullary
// predicates, so the requested/public representation is never normalized to
// sparse storage.
func NewSparseIntervalHashOriginal(manager *RelationManager, sorts []ast.ASTHandle, full bool) (*SparseIntervalSieveProduct, error) {
	finiteColumns, _, _, err := sparseIntervalProductColumns(manager, sorts)
	if err != nil {
		return nil, err
	}
	if len(finiteColumns) != len(sorts) {
		return nil, ErrSparseIntervalProduct
	}
	return newSparseIntervalSieveProduct(manager, sorts, full, false)
}

// NewSparseIntervalArithmeticOriginal constructs the concrete evaluator owner
// for an Int/Real original predicate. Its finite component is a full,
// transparent zero-column sparse table.
func NewSparseIntervalArithmeticOriginal(manager *RelationManager, sorts []ast.ASTHandle, full bool) (*SparseIntervalSieveProduct, error) {
	finiteColumns, _, _, err := sparseIntervalProductColumns(manager, sorts)
	if err != nil {
		return nil, err
	}
	if len(finiteColumns) != 0 {
		return nil, ErrSparseIntervalProduct
	}
	if len(sorts) == 0 {
		product, cause := newSparseIntervalSieveProduct(manager, sorts, full, false)
		if cause != nil {
			return nil, cause
		}
		// interval_relation owns the nullary truth value. The finite side is
		// the transparent zero-column identity, while the interval side is
		// empty/full according to the requested relation constructor.
		signature, cause := product.finite.Signature()
		if cause == nil {
			cause = fillTotalTable(product.finite, signature)
		}
		if cause == nil && !full {
			product.interval, cause = NewIntervalSieveEmpty(manager.manager, sorts)
		}
		if cause != nil {
			return nil, errors.Join(cause, product.Close())
		}
		product.intervalNullary = true
		product.intervalShape = sparseIntervalShapeRaw
		return product, nil
	}
	return newSparseIntervalSieveProduct(manager, sorts, full, false)
}

// NewSparseIntervalArithmeticOriginalFrom clones a borrowed interval seed into
// an independently owned arithmetic-original product. The finite component
// remains the full zero-column identity used by source T_FULL completion.
func NewSparseIntervalArithmeticOriginalFrom(manager *RelationManager, sorts []ast.ASTHandle, seed *IntervalRelation) (*SparseIntervalSieveProduct, error) {
	product, err := NewSparseIntervalArithmeticOriginal(manager, sorts, false)
	if err != nil {
		return nil, err
	}
	if seed == nil {
		return nil, errors.Join(ErrSparseIntervalProduct, product.Close())
	}
	arity, err := seed.Arity()
	if err != nil || int(arity) != len(sorts) {
		return nil, errors.Join(ErrSparseIntervalProduct, err, product.Close())
	}
	inner, err := seed.Clone()
	if err != nil {
		return nil, errors.Join(err, product.Close())
	}
	interval, err := newIntervalSieve(manager.manager, sorts, intervalSieveMask(manager.manager, sorts), inner)
	if err != nil {
		return nil, errors.Join(err, product.Close())
	}
	product.interval = interval
	return product, nil
}

// MatchesSorts validates the exact ordered signature used for a transactional
// seed replacement without exposing borrowed product internals.
func (p *SparseIntervalSieveProduct) MatchesSorts(sorts []ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	if len(sorts) != len(p.sorts) {
		return ErrSparseIntervalProduct
	}
	for i := range sorts {
		if sorts[i] != p.sorts[i] {
			return ErrSparseIntervalProduct
		}
	}
	return nil
}

// OwnedBy reports whether manager is the exact live relation manager that
// owns this product. Sharing an AST manager is not sufficient because encoded
// finite domains and teardown belong to the relation-manager instance.
func (p *SparseIntervalSieveProduct) OwnedBy(manager *RelationManager) bool {
	return manager != nil && p != nil && p.manager == manager && p.open() == nil
}

// FiniteSnapshot returns an owned exact sparse snapshot of a pure finite
// original. The product itself deliberately does not implement FiniteTable.
func (p *SparseIntervalSieveProduct) FiniteSnapshot() (*SparseTable, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if len(p.sorts) == 0 || len(p.finiteColumns) != len(p.sorts) {
		return nil, ErrSparseIntervalProduct
	}
	return p.finite.Clone()
}

// HashSnapshot returns an owned exact hashtable snapshot of a pure finite or
// nullary original. Rows are copied from the evaluator component; the product
// remains private and the observable representation stays hashtable-backed.
func (p *SparseIntervalSieveProduct) HashSnapshot() (*HashTable, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if p.intervalNullary || len(p.finiteColumns) != len(p.sorts) {
		return nil, ErrSparseIntervalProduct
	}
	signature, err := p.finite.Signature()
	if err != nil {
		return nil, err
	}
	result := NewHashTable(signature)
	facts, err := p.finite.Facts()
	if err != nil {
		return nil, errors.Join(err, result.Close())
	}
	for _, fact := range facts {
		if _, err = result.Add(fact); err != nil {
			return nil, errors.Join(err, result.Close())
		}
	}
	return result, nil
}

// ArithmeticSnapshot returns an owned interval snapshot of a pure arithmetic
// original.
func (p *SparseIntervalSieveProduct) ArithmeticSnapshot() (*IntervalRelation, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if len(p.finiteColumns) != 0 || (len(p.sorts) == 0 && !p.intervalNullary) {
		return nil, ErrSparseIntervalProduct
	}
	return p.interval.inner.Clone()
}
