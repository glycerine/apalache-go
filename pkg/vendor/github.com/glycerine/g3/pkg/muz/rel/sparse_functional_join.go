package rel

func sparseFunctionalJoinFact(left, right Fact, leftFirst, rightFirst int) Fact {
	result := make(Fact, 0, len(left)+len(right))
	result = append(result, left[:leftFirst]...)
	result = append(result, right[:rightFirst]...)
	result = append(result, left[leftFirst:]...)
	result = append(result, right[rightFirst:]...)
	return result
}

func (left *SparseTable) JoinFunctional(right *SparseTable, leftColumns, rightColumns []int) (*SparseTable, error) {
	if err := left.open(); err != nil {
		return nil, err
	}
	if err := right.open(); err != nil {
		return nil, err
	}
	if err := validateColumns(left.signature, right.signature, leftColumns, rightColumns); err != nil {
		return nil, err
	}
	if sparseJoinInvolvesFunctional(left, right, leftColumns, rightColumns) {
		return nil, ErrTable
	}
	signature, functional, err := sparseJoinFunctionalSignature(left, right)
	if err != nil {
		return nil, err
	}
	result, err := NewSparseTable(signature, functional)
	if err != nil {
		return nil, err
	}
	leftFacts, _ := left.Facts()
	rightFacts, _ := right.Facts()
	leftFirst := left.signature.Len() - left.functional
	rightFirst := right.signature.Len() - right.functional
	if err = sparseJoinTraversal(leftFacts, rightFacts, leftColumns, rightColumns, func(first, second Fact) error {
		_, addErr := result.Add(sparseFunctionalJoinFact(first, second, leftFirst, rightFirst))
		return addErr
	}); err != nil {
		return nil, err
	}
	return result, nil
}
