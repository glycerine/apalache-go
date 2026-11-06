package rel

func (plan *SparseIndexedNegationPlan) collectWithNegativeOuter(target *SparseTable, negativeFacts []Fact, marked map[string]bool, stats *SparseIndexedNegationStats) error {
	indexPlan, err := NewSparseKeyLookupPlan(target, plan.targetColumns)
	if err != nil {
		return err
	}
	index, indexStats, err := indexPlan.Build(target)
	if err != nil {
		return err
	}
	stats.IndexGroups = indexStats.Groups
	for _, negativeFact := range negativeFacts {
		stats.Probes++
		matches, err := index.Lookup(sparseIndexedJoinKey(negativeFact, plan.negativeColumns))
		if err != nil {
			return err
		}
		stats.Candidates += len(matches)
		for _, targetFact := range matches {
			marked[factKey(targetFact)] = true
		}
	}
	return nil
}
