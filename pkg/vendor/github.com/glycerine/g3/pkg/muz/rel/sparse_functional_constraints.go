package rel

func sparseSameFunctionalSignature(left, right *SparseTable) bool {
	return left != nil && right != nil && left.functional == right.functional && left.signature.equal(right.signature)
}

func sparseJoinInvolvesFunctional(left, right *SparseTable, leftColumns, rightColumns []int) bool {
	leftFirst := left.signature.Len() - left.functional
	rightFirst := right.signature.Len() - right.functional
	for i := range leftColumns {
		if leftColumns[i] >= leftFirst || rightColumns[i] >= rightFirst {
			return true
		}
	}
	return false
}

func sparseValidateFunctionalSuffix(table *SparseTable, suffix Fact) error {
	if len(suffix) != table.functional {
		return ErrTable
	}
	first := table.signature.Len() - table.functional
	for i, value := range suffix {
		if uint64(value) >= table.signature.domains[first+i] {
			return ErrTable
		}
	}
	return nil
}
