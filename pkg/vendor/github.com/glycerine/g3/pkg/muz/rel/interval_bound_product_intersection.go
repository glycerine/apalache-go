package rel

// Source: relation_manager::try_mk_default_filter_by_intersection_fn builds a
// join/project that removes every source column, then swaps the compatible
// filtered result into the target.

import "github.com/glycerine/g3/pkg/ast"

func (p *IntervalBoundProduct) FilterByIntersection(source *IntervalBoundProduct, targetColumns, sourceColumns []uint32) error {
	if p == nil || source == nil || p.interval == nil || source.interval == nil || p.bound == nil || source.bound == nil || len(targetColumns) != len(sourceColumns) {
		return ast.ErrASTDomain
	}
	targetArity, err := p.interval.Arity()
	if err != nil || int(targetArity) != len(p.bound.columns) {
		return ast.ErrASTDomain
	}
	sourceArity, err := source.interval.Arity()
	if err != nil || int(sourceArity) != len(source.bound.columns) {
		return ast.ErrASTDomain
	}
	removed := make([]uint32, sourceArity)
	for i := range removed {
		removed[i] = targetArity + uint32(i)
	}
	filtered, err := JoinProjectIntervalBoundProducts(p, source, targetColumns, sourceColumns, removed)
	if err != nil {
		return err
	}
	*p = *filtered
	return nil
}
