package rel

// Source: src/muz/rel/dl_sparse_table.cpp negated_join_fn.

func sparseNegatedJoinIndexable(plan *SparseNegatedJoinPlan) bool {
	firstFunctional := plan.firstSignature.Len() - plan.firstFunctional
	secondFunctional := plan.secondSignature.Len() - plan.secondFunctional
	for _, sourceColumn := range plan.sourceColumns {
		if sourceColumn < firstFunctional {
			continue
		}
		if sourceColumn < plan.firstSignature.Len() || sourceColumn-plan.firstSignature.Len() >= secondFunctional {
			return false
		}
	}
	for _, column := range plan.secondJoinColumns {
		if column >= secondFunctional {
			return false
		}
	}
	return true
}

func (plan *SparseNegatedJoinPlan) collectIndexedRemovals(targetFacts []Fact, first, second *SparseTable, removed *[]Fact, stats *SparseNegatedJoinStats) error {
	firstColumns := make([]int, 0, len(plan.sourceColumns))
	firstTargetColumns := make([]int, 0, len(plan.sourceColumns))
	secondColumns := make([]int, 0, len(plan.sourceColumns)+len(plan.secondJoinColumns))
	secondTargetColumns := make([]int, 0, len(plan.sourceColumns))
	for i, sourceColumn := range plan.sourceColumns {
		if sourceColumn < plan.firstSignature.Len() {
			firstTargetColumns = append(firstTargetColumns, plan.targetColumns[i])
			firstColumns = append(firstColumns, sourceColumn)
		} else {
			secondTargetColumns = append(secondTargetColumns, plan.targetColumns[i])
			secondColumns = append(secondColumns, sourceColumn-plan.firstSignature.Len())
		}
	}
	secondColumns = append(secondColumns, plan.secondJoinColumns...)
	firstPlan, err := NewSparseKeyLookupPlan(first, firstColumns)
	if err != nil {
		return err
	}
	firstIndex, _, err := firstPlan.Build(first)
	if err != nil {
		return err
	}
	secondPlan, err := NewSparseKeyLookupPlan(second, secondColumns)
	if err != nil {
		return err
	}
	secondIndex, _, err := secondPlan.Build(second)
	if err != nil {
		return err
	}
	firstKey := make(Fact, len(firstTargetColumns))
	secondKey := make(Fact, len(secondTargetColumns)+len(plan.firstJoinColumns))
	for _, target := range targetFacts {
		for i, column := range firstTargetColumns {
			firstKey[i] = target[column]
		}
		firstGroupIndex, found := firstIndex.byKey[factKey(firstKey)]
		if !found {
			continue
		}
		firstMatches := firstIndex.groups[firstGroupIndex-1].Rows
		stats.FirstProbes += len(firstMatches)
		matched := false
		for _, firstFact := range firstMatches {
			for i, column := range secondTargetColumns {
				secondKey[i] = target[column]
			}
			for i, column := range plan.firstJoinColumns {
				secondKey[len(secondTargetColumns)+i] = firstFact[column]
			}
			if secondGroupIndex, found := secondIndex.byKey[factKey(secondKey)]; found {
				stats.SecondProbes += len(secondIndex.groups[secondGroupIndex-1].Rows)
				matched = true
				break
			}
		}
		if matched {
			*removed = append(*removed, target)
		}
	}
	return nil
}
