package rel

func cloneSparseFacts(source []Fact) []Fact {
	result := make([]Fact, len(source))
	for i, fact := range source {
		result[i] = cloneFact(fact)
	}
	return result
}

func cloneSparseKeyGroup(group SparseKeyGroup) SparseKeyGroup {
	return SparseKeyGroup{Key: cloneFact(group.Key), Rows: cloneSparseFacts(group.Rows)}
}

func (snapshot *SparseKeyIndexSnapshot) Lookup(key Fact) ([]Fact, error) {
	if snapshot == nil || snapshot.plan == nil || snapshot.byKey == nil {
		return nil, ErrTable
	}
	if err := snapshot.plan.validateKey(key); err != nil {
		return nil, err
	}
	index := snapshot.byKey[factKey(key)]
	if index == 0 {
		return []Fact{}, nil
	}
	return cloneSparseFacts(snapshot.groups[index-1].Rows), nil
}

func (snapshot *SparseKeyIndexSnapshot) Contains(key Fact) (bool, error) {
	rows, err := snapshot.Lookup(key)
	return len(rows) != 0, err
}

func (snapshot *SparseKeyIndexSnapshot) Groups() []SparseKeyGroup {
	if snapshot == nil {
		return nil
	}
	result := make([]SparseKeyGroup, len(snapshot.groups))
	for i, group := range snapshot.groups {
		result[i] = cloneSparseKeyGroup(group)
	}
	return result
}
