package rel

// FilterByIntersection mirrors table_relation_plugin's positive intersection
// forwarding functor.
func (r *TableRelation) FilterByIntersection(source *TableRelation, targetColumns, sourceColumns []int) error {
	if err := r.open(); err != nil {
		return err
	}
	if source == nil || source.open() != nil || source.manager != r.manager {
		return ErrTableRelation
	}
	return r.table.FilterByIntersection(source.table, targetColumns, sourceColumns)
}
