package rel

func (table *SparseTable) ComplementFunctional(functionalSuffix Fact) (*SparseTable, error) {
	if err := table.open(); err != nil {
		return nil, err
	}
	if err := sparseValidateFunctionalSuffix(table, functionalSuffix); err != nil {
		return nil, err
	}
	first := table.signature.Len() - table.functional
	if first > 1 {
		return nil, ErrTable
	}
	result, err := NewSparseTable(table.signature, table.functional)
	if err != nil {
		return nil, err
	}
	if first == 0 {
		empty, emptyErr := table.Empty()
		if emptyErr != nil {
			return nil, emptyErr
		}
		if empty {
			_, err = result.Add(cloneFact(functionalSuffix))
		}
		return result, err
	}
	for value := uint64(0); value < table.signature.domains[0]; value++ {
		fact := append(Fact{Element(value)}, functionalSuffix...)
		found, containsErr := table.ContainsExact(fact)
		if containsErr != nil {
			return nil, containsErr
		}
		if !found {
			if _, err = result.Add(fact); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}
