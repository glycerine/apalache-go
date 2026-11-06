package rel

func (plan *SparseIndexedNegationPlan) Execute(target, negative *SparseTable) (SparseIndexedNegationStats, error) {
	stats := SparseIndexedNegationStats{}
	if err := plan.validateTables(target, negative); err != nil {
		return stats, err
	}
	targetFacts, err := target.Facts()
	if err != nil {
		return stats, err
	}
	negativeFacts, err := negative.Facts()
	if err != nil {
		return stats, err
	}
	stats.TargetRows, stats.NegativeRows = len(targetFacts), len(negativeFacts)
	if len(plan.targetColumns) == 0 {
		if len(negativeFacts) == 0 {
			return stats, nil
		}
		stats.ClearedProduct, stats.Removed = true, len(targetFacts)
		return stats, target.Reset()
	}
	marked := make(map[string]bool)
	stats.UsedNegativeOuter = len(targetFacts)/4 > len(negativeFacts)
	if stats.UsedNegativeOuter {
		err = plan.collectWithNegativeOuter(target, negativeFacts, marked, &stats)
	} else {
		err = plan.collectWithTargetOuter(negative, targetFacts, marked, &stats)
	}
	if err != nil {
		return stats, err
	}
	stats.Removed, err = removeSparseIndexedNegationFacts(target, targetFacts, marked)
	return stats, err
}
