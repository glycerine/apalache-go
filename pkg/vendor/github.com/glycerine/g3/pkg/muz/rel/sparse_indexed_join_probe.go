package rel

func (plan *SparseIndexedJoinPlan) executeIndexed(left, right *SparseTable, leftFacts, rightFacts []Fact, result *SparseTable, stats *SparseIndexedJoinStats) error {
	stats.IndexedLeft = len(leftFacts) > len(rightFacts)
	indexedTable, indexedColumns := right, plan.rightColumns
	probeFacts, probeColumns := leftFacts, plan.leftColumns
	if stats.IndexedLeft {
		indexedTable, indexedColumns = left, plan.leftColumns
		probeFacts, probeColumns = rightFacts, plan.rightColumns
	}
	indexPlan, err := NewSparseKeyLookupPlan(indexedTable, indexedColumns)
	if err != nil {
		return err
	}
	index, indexStats, err := indexPlan.Build(indexedTable)
	if err != nil {
		return err
	}
	stats.IndexGroups = indexStats.Groups
	for _, probe := range probeFacts {
		stats.Probes++
		matches, err := index.Lookup(sparseIndexedJoinKey(probe, probeColumns))
		if err != nil {
			return err
		}
		for _, match := range matches {
			leftFact, rightFact := probe, match
			if stats.IndexedLeft {
				leftFact, rightFact = match, probe
			}
			if err := plan.addResult(result, sparseIndexedJoinedFact(left, right, leftFact, rightFact), stats); err != nil {
				return err
			}
		}
	}
	return nil
}
