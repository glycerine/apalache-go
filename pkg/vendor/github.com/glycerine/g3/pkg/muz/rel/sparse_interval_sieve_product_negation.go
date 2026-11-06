package rel

// Source: dl_compiler.cpp stratified negation and
// sparse_table_plugin::mk_filter_by_negation_fn. This retained product path is
// valid only when both operands are exact finite table components.

func (p *SparseIntervalSieveProduct) FilterFiniteNegation(negative *SparseIntervalSieveProduct, targetColumns, negativeColumns []int) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if negative == nil || negative.open() != nil || p.manager != negative.manager || len(targetColumns) != len(negativeColumns) {
		return false, ErrSparseIntervalProduct
	}
	if p.intervalShape != sparseIntervalShapeAbsent || negative.intervalShape != sparseIntervalShapeAbsent || p.intervalNullary || negative.intervalNullary || len(p.finiteColumns) != len(p.sorts) || len(negative.finiteColumns) != len(negative.sorts) {
		return false, ErrSparseIntervalProduct
	}
	for index, targetColumn := range targetColumns {
		negativeColumn := negativeColumns[index]
		if targetColumn < 0 || targetColumn >= len(p.sorts) || negativeColumn < 0 || negativeColumn >= len(negative.sorts) || p.sorts[targetColumn] != negative.sorts[negativeColumn] {
			return false, ErrSparseIntervalProduct
		}
	}
	before, err := p.finite.Len()
	if err != nil {
		return false, err
	}
	if err = p.finite.FilterNegation(negative.finite, targetColumns, negativeColumns); err != nil {
		return false, err
	}
	after, err := p.finite.Len()
	return after != before, err
}
