package rel

func sparseJoinFunctionalSignature(left, right *SparseTable) (Signature, int, error) {
	leftFirst := left.signature.Len() - left.functional
	rightFirst := right.signature.Len() - right.functional
	domains := make([]uint64, 0, left.signature.Len()+right.signature.Len())
	domains = append(domains, left.signature.domains[:leftFirst]...)
	domains = append(domains, right.signature.domains[:rightFirst]...)
	domains = append(domains, left.signature.domains[leftFirst:]...)
	domains = append(domains, right.signature.domains[rightFirst:]...)
	signature, err := NewSignature(domains)
	return signature, left.functional + right.functional, err
}

func sparseProjectFunctionalCount(table *SparseTable, removed []int) (int, error) {
	if _, err := projectedSignature(table.signature, removed); err != nil {
		return 0, err
	}
	if len(removed) == 0 {
		return table.functional, nil
	}
	first := table.signature.Len() - table.functional
	if removed[0] < first {
		return 0, nil
	}
	return table.functional - len(removed), nil
}

func sparseProjectFact(fact Fact, removed []int) Fact {
	result := make(Fact, 0, len(fact)-len(removed))
	next := 0
	for column, value := range fact {
		if next < len(removed) && removed[next] == column {
			next++
			continue
		}
		result = append(result, value)
	}
	return result
}
