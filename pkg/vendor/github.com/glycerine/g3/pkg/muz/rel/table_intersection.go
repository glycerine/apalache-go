package rel

// FilterByIntersection keeps target rows whose selected columns match at least
// one source row. Target iteration order is preserved.
func (t *HashTable) FilterByIntersection(source *HashTable, targetColumns, sourceColumns []int) error {
	if err := validateTableIntersection(t, source, targetColumns, sourceColumns); err != nil {
		return err
	}
	if source.rows.Len() == 0 {
		return t.Reset()
	}
	if len(targetColumns) == 0 {
		return nil
	}
	index := buildTableIntersectionIndex(source, sourceColumns)
	for key, fact := range t.rows.All() {
		if _, found := index[tableIntersectionKey(fact, targetColumns)]; !found {
			t.rows.Del(key)
		}
	}
	return nil
}
