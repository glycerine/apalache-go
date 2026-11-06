package rel

func tableIntersectionKey(fact Fact, columns []int) string {
	key := make(Fact, len(columns))
	for index, column := range columns {
		key[index] = fact[column]
	}
	return factKey(key)
}

func buildTableIntersectionIndex(source *HashTable, columns []int) map[string]struct{} {
	index := make(map[string]struct{}, source.rows.Len())
	for _, fact := range source.rows.All() {
		index[tableIntersectionKey(fact, columns)] = struct{}{}
	}
	return index
}
