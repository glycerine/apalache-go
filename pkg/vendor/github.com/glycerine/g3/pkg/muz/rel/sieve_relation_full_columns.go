package rel

// Source: src/muz/rel/dl_sieve_relation.cpp
// sieve_relation_plugin::full(signature, inner_plugin).

import "errors"

// NewFullSieveWithColumns constructs a full outer relation while preserving
// the concrete selected-column layout. Unlike NewFullSieve, selected columns
// remain represented by the inner HashTable and are filled with their full
// finite product.
func NewFullSieveWithColumns(signature Signature, innerColumns []bool) (result *SieveRelation, resultErr error) {
	result, err := NewSieveRelation(signature, innerColumns)
	if err != nil {
		return nil, err
	}
	owned := result
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, owned.Close())
			result = nil
		}
	}()
	innerSignature, err := result.inner.Signature()
	if err != nil {
		return nil, err
	}
	if err = fillTotalTable(result.inner, innerSignature); err != nil {
		return nil, err
	}
	keep = true
	return result, nil
}

// EmptyLike is the concrete mk_empty(original) lifecycle operation. It keeps
// the selected-column layout but owns an independent empty inner relation.
func (r *SieveRelation) EmptyLike() (*SieveRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	return NewSieveRelation(r.signature, r.innerCols)
}
