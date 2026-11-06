package rel

func (table *SparseTable) RemoveFacts(facts []Fact) (int, error) {
	if err := table.open(); err != nil {
		return 0, err
	}
	removed := 0
	for _, fact := range facts {
		var changed bool
		var err error
		if table.functional != 0 {
			// Preserve source validation of the complete public row before
			// deleting by its unique nonfunctional prefix.
			if _, err = table.layout.pack(table.signature, fact); err == nil {
				firstFunctional := table.signature.Len() - table.functional
				changed, err = table.RemoveFunctionalKey(fact[:firstFunctional])
			}
		} else {
			changed, err = table.Remove(fact)
		}
		if err != nil {
			return removed, err
		}
		if changed {
			removed++
		}
	}
	return removed, nil
}
