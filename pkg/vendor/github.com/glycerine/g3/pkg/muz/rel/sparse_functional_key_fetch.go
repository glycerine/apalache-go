package rel

// FetchFunctionalKey looks up a functional row by its non-functional key and
// returns an owned copy of the stored suffix. It is the typed key-only form of
// sparse_table::fetch_fact and covers the source's zero-column key used by
// finite-product outer indexes.
func (table *SparseTable) FetchFunctionalKey(key Fact) (Fact, bool, error) {
	if err := table.open(); err != nil {
		return nil, false, err
	}
	firstFunctional := table.signature.Len() - table.functional
	if table.functional == 0 || len(key) != firstFunctional {
		return nil, false, ErrTable
	}
	probe := make(Fact, table.signature.Len())
	for column, value := range key {
		domain, err := table.signature.Domain(column)
		if err != nil || uint64(value) >= domain {
			return nil, false, ErrTable
		}
		probe[column] = value
	}
	packed, err := table.layout.pack(table.signature, probe)
	if err != nil {
		return nil, false, err
	}
	index, found := table.data.find(packed)
	if !found {
		return nil, false, nil
	}
	stored := table.layout.unpack(table.data.entries[index])
	return append(Fact(nil), stored[firstFunctional:]...), true, nil
}
