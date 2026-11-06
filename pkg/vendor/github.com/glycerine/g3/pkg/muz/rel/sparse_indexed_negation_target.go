package rel

func (plan *SparseIndexedNegationPlan) collectWithTargetOuter(negative *SparseTable, targetFacts []Fact, marked map[string]bool, stats *SparseIndexedNegationStats) error {
	indexPlan, err := NewSparseKeyLookupPlan(negative, plan.negativeColumns)
	if err != nil {
		return err
	}
	index, indexStats, err := indexPlan.Build(negative)
	if err != nil {
		return err
	}
	stats.IndexGroups = indexStats.Groups
	for _, targetFact := range targetFacts {
		stats.Probes++
		matches, err := index.Lookup(sparseIndexedJoinKey(targetFact, plan.targetColumns))
		if err != nil {
			return err
		}
		stats.Candidates += len(matches)
		if len(matches) != 0 {
			marked[factKey(targetFact)] = true
		}
	}
	return nil
}
