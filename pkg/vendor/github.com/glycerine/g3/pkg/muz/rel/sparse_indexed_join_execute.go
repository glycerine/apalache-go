package rel

func (plan *SparseIndexedJoinPlan) Execute(left, right *SparseTable) (*SparseTable, SparseIndexedJoinStats, error) {
	stats := SparseIndexedJoinStats{}
	if err := plan.validateTables(left, right); err != nil {
		return nil, stats, err
	}
	leftFacts, err := left.Facts()
	if err != nil {
		return nil, stats, err
	}
	rightFacts, err := right.Facts()
	if err != nil {
		return nil, stats, err
	}
	stats.LeftRows, stats.RightRows = len(leftFacts), len(rightFacts)
	result, err := NewSparseTable(plan.resultSignature, plan.resultFunctional)
	if err != nil {
		return nil, stats, err
	}
	if len(plan.leftColumns) == 0 {
		stats.Product = true
		err = plan.executeProduct(left, right, leftFacts, rightFacts, result, &stats)
	} else {
		err = plan.executeIndexed(left, right, leftFacts, rightFacts, result, &stats)
	}
	if err != nil {
		_ = result.Close()
		return nil, stats, err
	}
	return result, stats, nil
}

func (plan *SparseIndexedJoinPlan) addResult(result *SparseTable, joined Fact, stats *SparseIndexedJoinStats) error {
	stats.CandidateRows++
	inserted, err := result.Add(plan.projectFact(joined))
	if inserted {
		stats.Inserted++
	}
	return err
}
