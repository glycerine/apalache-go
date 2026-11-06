package rel

import "github.com/glycerine/g3/pkg/ast"

// FilterByIntersection implements relation_manager's live default
// intersection filter for the concrete bound-vector relation. The source
// joins the operands, projects away every source column, resets the target,
// and unions the filtered value back because vector_relation cannot swap.
func (r *BoundRelation) FilterByIntersection(source *BoundRelation, targetColumns, sourceColumns []uint32) error {
	if r == nil || source == nil || len(targetColumns) != len(sourceColumns) {
		return ast.ErrASTDomain
	}
	joined, err := JoinBoundRelations(r, source, targetColumns, sourceColumns)
	if err != nil {
		return err
	}
	targetArity := len(r.columns)
	removed := make([]uint32, len(source.columns))
	for index := range removed {
		removed[index] = uint32(targetArity + index)
	}
	filtered, err := joined.Project(removed)
	if err != nil {
		return err
	}
	r.reset()
	_, err = r.Union(filtered)
	return err
}
