package rel

import "github.com/glycerine/g3/pkg/ast"

// FilterByIntersection implements relation_manager's default intersection
// filter for interval vector relations: join target and source, project every
// source column, reset the target, then union the filtered relation into it.
// vector_relation supplies swap but not can_swap, so the native relation
// manager takes this union branch rather than its swap fast path.
func (r *IntervalRelation) FilterByIntersection(source *IntervalRelation, targetColumns, sourceColumns []uint32) error {
	if r == nil || source == nil || r.values == nil || source.values == nil || len(targetColumns) != len(sourceColumns) {
		return ast.ErrASTDomain
	}
	joined, err := JoinIntervalRelations(r, source, targetColumns, sourceColumns)
	if err != nil {
		return err
	}
	targetArity := len(r.values.columns)
	removed := make([]uint32, len(source.values.columns))
	for index := range removed {
		removed[index] = uint32(targetArity + index)
	}
	filtered, err := joined.Project(removed)
	if err != nil {
		return err
	}
	r.values.setEmpty()
	_, err = unionIntervalColumns(r.values, filtered.values, false)
	return err
}
