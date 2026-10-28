package apalache

import "fmt"

// setDeduplicationPredicates returns one Boolean cell per arena edge. Predicate
// i is true exactly when edge i is active and no earlier selected predicate
// denotes an equal element. This is the shared SetOps.dedup contract used by
// cardinality and FoldSet in the Scala rewriter.
func (rewriter *StratifiedRewriter) setDeduplicationPredicates(
	scope RewriterScope, set ArenaCell,
) (RewriterScope, []ArenaCell, error) {
	if !isFiniteSetCellType(set.Type) {
		return scope, nil, fmt.Errorf("deduplication requires a finite set, found %s", set.Type)
	}
	pointers := scope.Arena.GetHas(set)
	predicates := make([]ArenaCell, len(pointers))
	for index, pointer := range pointers {
		conditions := []Expr{pointer.ToSMT()}
		for previous := 0; previous < index; previous++ {
			equal, err := rewriter.equality.Equal(scope.Arena,
				pointer.Element(), pointers[previous].Element())
			if err != nil {
				return scope, nil, err
			}
			conditions = append(conditions, booleanOr(
				MustOperExpr(Op("NOT"), Typed(BoolT1), predicates[previous].NameExpr()),
				MustOperExpr(Op("NOT"), Typed(BoolT1), equal)))
		}
		scope, predicates[index] = rewriter.bindBoolean(scope, booleanAnd(conditions...))
	}
	return scope, predicates, nil
}
