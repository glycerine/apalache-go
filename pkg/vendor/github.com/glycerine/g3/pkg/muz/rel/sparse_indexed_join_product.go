package rel

func (plan *SparseIndexedJoinPlan) executeProduct(left, right *SparseTable, leftFacts, rightFacts []Fact, result *SparseTable, stats *SparseIndexedJoinStats) error {
	// The source keeps the smaller product operand in the inner loop.
	leftOuter := len(leftFacts) > len(rightFacts)
	if leftOuter {
		for _, leftFact := range leftFacts {
			for _, rightFact := range rightFacts {
				stats.Probes++
				if err := plan.addResult(result, sparseIndexedJoinedFact(left, right, leftFact, rightFact), stats); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, rightFact := range rightFacts {
		for _, leftFact := range leftFacts {
			stats.Probes++
			if err := plan.addResult(result, sparseIndexedJoinedFact(left, right, leftFact, rightFact), stats); err != nil {
				return err
			}
		}
	}
	return nil
}
