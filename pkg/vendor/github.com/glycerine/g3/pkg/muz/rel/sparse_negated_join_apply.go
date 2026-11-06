package rel

func (plan *SparseNegatedJoinPlan) Apply(target, first, second *SparseTable) (SparseNegatedJoinStats, error) {
	stats := SparseNegatedJoinStats{}
	if err := plan.validateTables(target, first, second); err != nil {
		return stats, err
	}
	targetFacts, err := target.Facts()
	if err != nil {
		return stats, err
	}
	firstFacts, err := first.Facts()
	if err != nil {
		return stats, err
	}
	secondFacts, err := second.Facts()
	if err != nil {
		return stats, err
	}
	stats.TargetRows, stats.FirstRows, stats.SecondRows = len(targetFacts), len(firstFacts), len(secondFacts)
	remove := make([]Fact, 0)
	if sparseNegatedJoinIndexable(plan) {
		if err = plan.collectIndexedRemovals(targetFacts, first, second, &remove, &stats); err != nil {
			return stats, err
		}
		stats.Removed, err = target.RemoveFacts(remove)
		return stats, err
	}
	for _, targetFact := range targetFacts {
		matched := false
		for _, firstFact := range firstFacts {
			stats.FirstProbes++
			if !plan.targetMatchesFirst(targetFact, firstFact) {
				continue
			}
			for _, secondFact := range secondFacts {
				stats.SecondProbes++
				if plan.sourcesJoin(firstFact, secondFact) && plan.targetMatchesSecond(targetFact, secondFact) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if matched {
			remove = append(remove, targetFact)
		}
	}
	stats.Removed, err = target.RemoveFacts(remove)
	return stats, err
}
