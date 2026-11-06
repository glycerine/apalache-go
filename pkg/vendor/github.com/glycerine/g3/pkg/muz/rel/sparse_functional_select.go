package rel

func (table *SparseTable) SelectEqualProjectFunctional(value Element, column int) (*SparseTable, error) {
	if err := table.open(); err != nil {
		return nil, err
	}
	if column < 0 || column >= table.signature.Len() || column >= table.signature.Len()-table.functional || table.signature.Len() == 1 {
		return nil, ErrTable
	}
	signature, err := projectedSignature(table.signature, []int{column})
	if err != nil {
		return nil, err
	}
	functional, err := sparseProjectFunctionalCount(table, []int{column})
	if err != nil {
		return nil, err
	}
	result, err := NewSparseTable(signature, functional)
	if err != nil {
		return nil, err
	}
	facts, _ := table.Facts()
	for _, fact := range facts {
		if fact[column] == value {
			if _, err = result.Add(sparseProjectFact(fact, []int{column})); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}
