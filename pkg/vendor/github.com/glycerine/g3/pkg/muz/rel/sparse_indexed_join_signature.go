package rel

func (plan *SparseIndexedJoinPlan) projectFact(fact Fact) Fact {
	result := make(Fact, 0, len(fact)-len(plan.removed))
	removedAt := 0
	for column, value := range fact {
		if removedAt < len(plan.removed) && plan.removed[removedAt] == column {
			removedAt++
			continue
		}
		result = append(result, value)
	}
	return result
}

func sparseIndexedJoinKey(fact Fact, columns []int) Fact {
	key := make(Fact, len(columns))
	for i, column := range columns {
		key[i] = fact[column]
	}
	return key
}

func sparseIndexedJoinedFact(left, right *SparseTable, leftFact, rightFact Fact) Fact {
	leftFirst := left.signature.Len() - left.functional
	rightFirst := right.signature.Len() - right.functional
	return sparseFunctionalJoinFact(leftFact, rightFact, leftFirst, rightFirst)
}
