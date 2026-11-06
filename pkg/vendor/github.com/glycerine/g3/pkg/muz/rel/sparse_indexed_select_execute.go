package rel

func (plan *SparseIndexedSelectPlan) Execute(table *SparseTable) (*SparseTable, SparseIndexedSelectStats, error) {
	stats := SparseIndexedSelectStats{}
	if err := plan.validateTable(table); err != nil {
		return nil, stats, err
	}
	rows, err := table.Len()
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = rows
	indexPlan, err := NewSparseKeyLookupPlan(table, []int{plan.column})
	if err != nil {
		return nil, stats, err
	}
	index, indexStats, err := indexPlan.Build(table)
	if err != nil {
		return nil, stats, err
	}
	stats.IndexGroups, stats.FullSignature = indexStats.Groups, indexStats.FullSignature
	matches, err := index.Lookup(Fact{plan.value})
	if err != nil {
		return nil, stats, err
	}
	stats.Matches = len(matches)
	result, err := plan.newResult()
	if err != nil {
		return nil, stats, err
	}
	for _, fact := range matches {
		inserted, addErr := result.Add(sparseProjectFact(fact, []int{plan.column}))
		if addErr != nil {
			_ = result.Close()
			return nil, stats, addErr
		}
		if inserted {
			stats.Inserted++
		}
	}
	return result, stats, nil
}
