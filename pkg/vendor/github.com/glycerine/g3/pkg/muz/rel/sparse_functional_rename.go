package rel

func (table *SparseTable) RenameFunctional(cycle []int) (*SparseTable, error) {
	if err := table.open(); err != nil {
		return nil, err
	}
	first := table.signature.Len() - table.functional
	seen := make(map[int]bool, len(cycle))
	functionalSide := false
	for i, column := range cycle {
		if column < 0 || column >= table.signature.Len() || seen[column] {
			return nil, ErrTable
		}
		seen[column] = true
		if i == 0 {
			functionalSide = column >= first
		} else if (column >= first) != functionalSide {
			return nil, ErrTable
		}
	}
	domains := table.signature.Domains()
	if len(cycle) > 1 {
		temporary := domains[cycle[0]]
		for i := 1; i < len(cycle); i++ {
			domains[cycle[i-1]] = domains[cycle[i]]
		}
		domains[cycle[len(cycle)-1]] = temporary
	}
	signature, err := NewSignature(domains)
	if err != nil {
		return nil, err
	}
	result, err := NewSparseTable(signature, table.functional)
	if err != nil {
		return nil, err
	}
	facts, _ := table.Facts()
	for _, fact := range facts {
		renamed := cloneFact(fact)
		if len(cycle) > 1 {
			temporary := renamed[cycle[0]]
			for i := 1; i < len(cycle); i++ {
				renamed[cycle[i-1]] = renamed[cycle[i]]
			}
			renamed[cycle[len(cycle)-1]] = temporary
		}
		if _, err = result.Add(renamed); err != nil {
			return nil, err
		}
	}
	return result, nil
}
