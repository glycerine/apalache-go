package rel

// Source: src/muz/rel/dl_sieve_relation.cpp join_fn, transformer_fn,
// union_fn, filter_fn, and their concrete operation factories.

import "fmt"

func validateRemovedColumns(count int, removed []int) ([]bool, error) {
	flags := make([]bool, count)
	previous := -1
	for _, column := range removed {
		if column <= previous || column < 0 || column >= count {
			return nil, fmt.Errorf("%w: removed columns must be increasing and in range", ErrSieve)
		}
		flags[column] = true
		previous = column
	}
	return flags, nil
}

func sieveCyclePermutation(count int, cycle []int) ([]int, error) {
	permutation := make([]int, count)
	for i := range permutation {
		permutation[i] = i
	}
	if len(cycle) < 2 {
		return permutation, nil
	}
	seen := make([]bool, count)
	for _, column := range cycle {
		if column < 0 || column >= count || seen[column] {
			return nil, fmt.Errorf("%w: invalid rename cycle", ErrSieve)
		}
		seen[column] = true
	}
	for i := 1; i < len(cycle); i++ {
		permutation[cycle[i-1]] = cycle[i]
	}
	permutation[cycle[len(cycle)-1]] = cycle[0]
	return permutation, nil
}

// Join concatenates both outer signatures. Equality pairs are forwarded only
// when both endpoints are retained; an ignored endpoint makes the equality an
// intentional over-approximation in the source sieve.
func (r *SieveRelation) Join(other *SieveRelation, leftColumns, rightColumns []int) (*SieveRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if other == nil || other.open() != nil || len(leftColumns) != len(rightColumns) {
		return nil, ErrSieve
	}
	if err := validateColumns(r.signature, other.signature, leftColumns, rightColumns); err != nil {
		return nil, err
	}
	innerLeft := make([]int, 0, len(leftColumns))
	innerRight := make([]int, 0, len(rightColumns))
	for i := range leftColumns {
		left, right := leftColumns[i], rightColumns[i]
		if r.IsInnerColumn(left) && other.IsInnerColumn(right) {
			innerLeft = append(innerLeft, r.sigToInner[left])
			innerRight = append(innerRight, other.sigToInner[right])
		}
	}
	joined, err := r.inner.Join(other.inner, innerLeft, innerRight)
	if err != nil {
		return nil, err
	}
	domains := append(r.signature.Domains(), other.signature.domains...)
	signature, err := NewSignature(domains)
	if err != nil {
		return nil, err
	}
	mask := append(append([]bool(nil), r.innerCols...), other.innerCols...)
	return AdoptSieveRelation(signature, mask, joined)
}

func (r *SieveRelation) Project(removed []int) (*SieveRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	flags, err := validateRemovedColumns(r.signature.Len(), removed)
	if err != nil {
		return nil, err
	}
	innerRemoved := make([]int, 0, len(removed))
	for _, column := range removed {
		if r.IsInnerColumn(column) {
			innerRemoved = append(innerRemoved, r.sigToInner[column])
		}
	}
	projected, err := r.inner.Project(innerRemoved)
	if err != nil {
		return nil, err
	}
	domains := make([]uint64, 0, r.signature.Len()-len(removed))
	mask := make([]bool, 0, r.signature.Len()-len(removed))
	for column, domain := range r.signature.domains {
		if !flags[column] {
			domains = append(domains, domain)
			mask = append(mask, r.innerCols[column])
		}
	}
	signature, err := NewSignature(domains)
	if err != nil {
		return nil, err
	}
	return AdoptSieveRelation(signature, mask, projected)
}

func (r *SieveRelation) RenameCycle(cycle []int) (*SieveRelation, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	permutation, err := sieveCyclePermutation(r.signature.Len(), cycle)
	if err != nil {
		return nil, err
	}
	domains := make([]uint64, len(permutation))
	mask := make([]bool, len(permutation))
	innerPermutation := make([]int, 0, len(r.innerToSig))
	for destination, source := range permutation {
		domains[destination] = r.signature.domains[source]
		mask[destination] = r.innerCols[source]
		if mask[destination] {
			innerPermutation = append(innerPermutation, r.sigToInner[source])
		}
	}
	renamed, err := r.inner.Permute(innerPermutation)
	if err != nil {
		return nil, err
	}
	signature, err := NewSignature(domains)
	if err != nil {
		return nil, err
	}
	return AdoptSieveRelation(signature, mask, renamed)
}

func (r *SieveRelation) sameLayout(other *SieveRelation) bool {
	if r == nil || other == nil || !r.signature.equal(other.signature) || len(r.innerCols) != len(other.innerCols) {
		return false
	}
	for i := range r.innerCols {
		if r.innerCols[i] != other.innerCols[i] {
			return false
		}
	}
	return true
}

func (r *SieveRelation) UnionFrom(source, delta *SieveRelation) error {
	if err := r.open(); err != nil {
		return err
	}
	if source == nil || source.open() != nil || !r.sameLayout(source) {
		return fmt.Errorf("%w: union sieve layouts differ", ErrSieve)
	}
	var innerDelta *HashTable
	if delta != nil {
		if delta.open() != nil || !r.sameLayout(delta) {
			return fmt.Errorf("%w: delta sieve layout differs", ErrSieve)
		}
		innerDelta = delta.inner
	}
	return r.inner.Union(source.inner, innerDelta)
}

func (r *SieveRelation) FilterEqual(value Element, column int) error {
	if err := r.open(); err != nil {
		return err
	}
	if column < 0 || column >= r.signature.Len() {
		return fmt.Errorf("%w: filter column out of range", ErrSieve)
	}
	if !r.IsInnerColumn(column) {
		return nil
	}
	return r.inner.FilterEqual(value, r.sigToInner[column])
}

func (r *SieveRelation) FilterIdentical(columns []int) error {
	if err := r.open(); err != nil {
		return err
	}
	innerColumns := make([]int, 0, len(columns))
	for _, column := range columns {
		if column < 0 || column >= r.signature.Len() {
			return fmt.Errorf("%w: identical-filter column out of range", ErrSieve)
		}
		if r.IsInnerColumn(column) {
			innerColumns = append(innerColumns, r.sigToInner[column])
		}
	}
	if len(innerColumns) < 2 {
		return nil
	}
	return r.inner.FilterIdentical(innerColumns)
}

// FilterByNegation forwards equalities that are meaningful in both inner
// relations. If a full target coordinate would have to match a retained
// negated coordinate, the source deliberately performs the identity operation
// rather than removing possibly valid target rows.
func (r *SieveRelation) FilterByNegation(negated *SieveRelation, targetColumns, negatedColumns []int) error {
	if err := r.open(); err != nil {
		return err
	}
	if negated == nil || negated.open() != nil || len(targetColumns) != len(negatedColumns) {
		return ErrSieve
	}
	if err := validateColumns(r.signature, negated.signature, targetColumns, negatedColumns); err != nil {
		return err
	}
	innerTarget := make([]int, 0, len(targetColumns))
	innerNegated := make([]int, 0, len(negatedColumns))
	for i := range targetColumns {
		targetInner := r.IsInnerColumn(targetColumns[i])
		negatedInner := negated.IsInnerColumn(negatedColumns[i])
		switch {
		case !targetInner && negatedInner:
			return nil
		case negatedInner:
			innerTarget = append(innerTarget, r.sigToInner[targetColumns[i]])
			innerNegated = append(innerNegated, negated.sigToInner[negatedColumns[i]])
		default:
			// A full negated coordinate can always match; omit this equality.
		}
	}
	return r.inner.FilterByNegation(negated.inner, innerTarget, innerNegated)
}
