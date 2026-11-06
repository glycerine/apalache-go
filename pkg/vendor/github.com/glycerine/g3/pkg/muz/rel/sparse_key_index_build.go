package rel

func (plan *SparseKeyLookupPlan) keyForFact(fact Fact) Fact {
	key := make(Fact, len(plan.columns))
	for i, column := range plan.columns {
		key[i] = fact[column]
	}
	return key
}

func (plan *SparseKeyLookupPlan) Build(table *SparseTable) (*SparseKeyIndexSnapshot, SparseKeyIndexStats, error) {
	stats := SparseKeyIndexStats{}
	if err := plan.validateTable(table); err != nil {
		return nil, stats, err
	}
	facts, err := table.Facts()
	if err != nil {
		return nil, stats, err
	}
	snapshot := &SparseKeyIndexSnapshot{plan: plan.Clone(), byKey: make(map[string]int)}
	for _, fact := range facts {
		key := plan.keyForFact(fact)
		encoded := factKey(key)
		index, found := snapshot.byKey[encoded]
		if !found {
			index = len(snapshot.groups)
			snapshot.byKey[encoded] = index + 1
			snapshot.groups = append(snapshot.groups, SparseKeyGroup{Key: key})
		} else {
			index--
		}
		snapshot.groups[index].Rows = append(snapshot.groups[index].Rows, cloneFact(fact))
	}
	stats.Rows = len(facts)
	stats.Groups = len(snapshot.groups)
	stats.KeyColumns = len(plan.columns)
	stats.FullSignature = plan.isFullSignature()
	return snapshot, stats, nil
}
