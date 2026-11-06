package rel

// Source: src/muz/rel/dl_sieve_relation.cpp
// sieve_relation_plugin::negation_filter_fn and
// mk_filter_by_negation_fn, instantiated for the retained concrete
// TableRelation specialization.

// FilterByTableNegation removes sieve target rows matching an ordinary finite
// negative table. If a full target column is paired with an ordinary column,
// source deliberately performs the identity operation: the inner sieve row
// cannot soundly be removed based on an unrepresented target coordinate.
func (r *SieveRelation) FilterByTableNegation(negative *TableRelation, targetColumns, negativeColumns []int) error {
	if err := r.open(); err != nil {
		return err
	}
	if negative == nil || negative.open() != nil || len(targetColumns) != len(negativeColumns) {
		return ErrSieve
	}
	negativeSignature, err := negative.table.Signature()
	if err != nil {
		return err
	}
	if err = validateColumns(r.signature, negativeSignature, targetColumns, negativeColumns); err != nil {
		return err
	}
	innerTarget := make([]int, 0, len(targetColumns))
	innerNegative := make([]int, 0, len(negativeColumns))
	for index, targetColumn := range targetColumns {
		if !r.IsInnerColumn(targetColumn) {
			return nil
		}
		innerTarget = append(innerTarget, r.sigToInner[targetColumn])
		innerNegative = append(innerNegative, negativeColumns[index])
	}
	return r.inner.FilterByNegation(negative.table, innerTarget, innerNegative)
}

// FilterTableByNegation removes ordinary finite target rows matching this
// negative sieve. Equalities ending at a full negative column are omitted:
// that coordinate can always match some value in its full domain.
func (r *SieveRelation) FilterTableByNegation(target *TableRelation, targetColumns, negativeColumns []int) error {
	if err := r.open(); err != nil {
		return err
	}
	if target == nil || target.open() != nil || len(targetColumns) != len(negativeColumns) {
		return ErrSieve
	}
	targetSignature, err := target.table.Signature()
	if err != nil {
		return err
	}
	if err = validateColumns(targetSignature, r.signature, targetColumns, negativeColumns); err != nil {
		return err
	}
	innerTarget := make([]int, 0, len(targetColumns))
	innerNegative := make([]int, 0, len(negativeColumns))
	for index, negativeColumn := range negativeColumns {
		if !r.IsInnerColumn(negativeColumn) {
			continue
		}
		innerTarget = append(innerTarget, targetColumns[index])
		innerNegative = append(innerNegative, r.sigToInner[negativeColumn])
	}
	return target.table.FilterByNegation(r.inner, innerTarget, innerNegative)
}
