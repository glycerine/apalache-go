package rel

// Source: src/muz/rel/dl_relation_manager.cpp default_table_join_project_fn.

// JoinProject applies the source default table join/project composition. The
// joined table is a temporary owner and is released after projection, matching
// the auxiliary table lifetime in default_table_join_project_fn.
func (t *HashTable) JoinProject(other *HashTable, leftColumns, rightColumns, removed []int) (*HashTable, error) {
	joined, err := t.Join(other, leftColumns, rightColumns)
	if err != nil {
		return nil, err
	}
	defer joined.Close()
	return joined.Project(removed)
}
