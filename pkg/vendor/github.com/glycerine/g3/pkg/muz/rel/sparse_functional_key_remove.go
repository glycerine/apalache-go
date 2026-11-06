package rel

// RemoveFunctionalKey removes the row identified by its nonfunctional prefix.
// sparse_table::remove_fact hashes only this unique prefix even though its
// public entry point receives a complete row.
func (table *SparseTable) RemoveFunctionalKey(key Fact) (bool, error) {
	if err := table.open(); err != nil {
		return false, err
	}
	if table.functional <= 0 {
		return false, ErrTable
	}
	firstFunctional := table.signature.Len() - table.functional
	if len(key) != firstFunctional {
		return false, ErrTable
	}
	packed := make([]byte, table.layout.entryBytes)
	for column, value := range key {
		if uint64(value) >= table.signature.domains[column] {
			return false, ErrTable
		}
		layout := table.layout.columns[column]
		for bit := uint32(0); bit < layout.length; bit++ {
			if uint64(value)>>bit&1 != 0 {
				position := layout.offset + bit
				packed[position/8] |= 1 << (position % 8)
			}
		}
	}
	return table.data.remove(packed), nil
}
