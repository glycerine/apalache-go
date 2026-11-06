package rel

func (table *SparseTable) ContainsExact(fact Fact) (bool, error) {
	return table.Contains(fact)
}

func (table *SparseTable) FetchFact(fact Fact) (bool, error) {
	if err := table.open(); err != nil {
		return false, err
	}
	packed, err := table.layout.pack(table.signature, fact)
	if err != nil {
		return false, err
	}
	index, found := table.data.find(packed)
	if !found {
		return false, nil
	}
	stored := table.layout.unpack(table.data.entries[index])
	copy(fact[table.signature.Len()-table.functional:], stored[table.signature.Len()-table.functional:])
	return true, nil
}

func (table *SparseTable) SuggestFact(fact Fact) (bool, error) {
	if found, err := table.FetchFact(fact); err != nil || found {
		return false, err
	}
	return table.Add(fact)
}

func (table *SparseTable) EnsureFact(fact Fact) error {
	if err := table.open(); err != nil {
		return err
	}
	if table.functional == 0 {
		_, err := table.Add(fact)
		return err
	}
	packed, err := table.layout.pack(table.signature, fact)
	if err != nil {
		return err
	}
	if index, found := table.data.find(packed); found {
		// Functional updates preserve the source row's physical position and
		// leave indexes over the unchanged non-functional key valid.
		copy(table.data.entries[index], packed)
		return nil
	}
	table.data.insert(packed)
	return nil
}
