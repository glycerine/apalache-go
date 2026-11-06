package rel

func (left *SparseTable) JoinProjectFunctionalPreserving(right *SparseTable, leftColumns, rightColumns, removed []int) (*SparseTable, error) {
	if err := left.open(); err != nil {
		return nil, err
	}
	if err := right.open(); err != nil {
		return nil, err
	}
	if err := validateColumns(left.signature, right.signature, leftColumns, rightColumns); err != nil {
		return nil, err
	}
	joined, err := left.JoinFunctional(right, leftColumns, rightColumns)
	if err != nil {
		return nil, err
	}
	defer joined.Close()
	signature, err := projectedSignature(joined.signature, removed)
	if err != nil {
		return nil, err
	}
	functional, err := sparseJoinProjectFunctionalCount(left, right, leftColumns, rightColumns, removed)
	if err != nil {
		return nil, err
	}
	result, err := NewSparseTable(signature, functional)
	if err != nil {
		return nil, err
	}
	facts, err := joined.Facts()
	if err != nil {
		return nil, err
	}
	for _, fact := range facts {
		if _, err = result.Add(sparseProjectFact(fact, removed)); err != nil {
			return nil, err
		}
	}
	return result, nil
}
