package rel

// Source: src/muz/rel/dl_sieve_relation.cpp
// sieve_relation_plugin::join_fn and mk_join_fn, instantiated for the retained
// concrete TableRelation specialization.

import "errors"

// JoinTableRelation joins a sieve on the left with an ordinary finite table.
// Equalities involving a full sieve column are omitted, matching the source
// over-approximation. The returned sieve owns an independent joined table.
func (r *SieveRelation) JoinTableRelation(other *TableRelation, leftColumns, rightColumns []int) (*SieveRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if other == nil || other.open() != nil || len(leftColumns) != len(rightColumns) {
		return nil, ErrSieve
	}
	otherSignature, err := other.table.Signature()
	if err != nil {
		return nil, err
	}
	if err = validateColumns(r.signature, otherSignature, leftColumns, rightColumns); err != nil {
		return nil, err
	}
	innerLeft := make([]int, 0, len(leftColumns))
	innerRight := make([]int, 0, len(rightColumns))
	for index, left := range leftColumns {
		if r.IsInnerColumn(left) {
			innerLeft = append(innerLeft, r.sigToInner[left])
			innerRight = append(innerRight, rightColumns[index])
		}
	}
	joined, err := r.inner.Join(other.table, innerLeft, innerRight)
	if err != nil {
		return nil, err
	}
	domains := append(r.signature.Domains(), otherSignature.domains...)
	signature, err := NewSignature(domains)
	if err != nil {
		return nil, errors.Join(err, joined.Close())
	}
	mask := append([]bool(nil), r.innerCols...)
	for range otherSignature.domains {
		mask = append(mask, true)
	}
	result, err := AdoptSieveRelation(signature, mask, joined)
	if err != nil {
		return nil, errors.Join(err, joined.Close())
	}
	return result, nil
}

// JoinFromTableRelation joins an ordinary finite table on the left with this
// sieve on the right.
func (r *SieveRelation) JoinFromTableRelation(left *TableRelation, leftColumns, rightColumns []int) (*SieveRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if left == nil || left.open() != nil || len(leftColumns) != len(rightColumns) {
		return nil, ErrSieve
	}
	leftSignature, err := left.table.Signature()
	if err != nil {
		return nil, err
	}
	if err = validateColumns(leftSignature, r.signature, leftColumns, rightColumns); err != nil {
		return nil, err
	}
	innerLeft := make([]int, 0, len(leftColumns))
	innerRight := make([]int, 0, len(rightColumns))
	for index, right := range rightColumns {
		if r.IsInnerColumn(right) {
			innerLeft = append(innerLeft, leftColumns[index])
			innerRight = append(innerRight, r.sigToInner[right])
		}
	}
	joined, err := left.table.Join(r.inner, innerLeft, innerRight)
	if err != nil {
		return nil, err
	}
	domains := append(leftSignature.Domains(), r.signature.domains...)
	signature, err := NewSignature(domains)
	if err != nil {
		return nil, errors.Join(err, joined.Close())
	}
	mask := make([]bool, leftSignature.Len(), leftSignature.Len()+len(r.innerCols))
	for index := range mask {
		mask[index] = true
	}
	mask = append(mask, r.innerCols...)
	result, err := AdoptSieveRelation(signature, mask, joined)
	if err != nil {
		return nil, errors.Join(err, joined.Close())
	}
	return result, nil
}
