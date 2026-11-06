package rel

// Source: src/muz/rel/dl_sieve_relation.cpp
// sieve_relation_plugin::mk_rename_fn, for the IntervalRelation inner kind.

import "github.com/glycerine/g3/pkg/ast"

// Rename rotates outer columns using sieve-relation cycle semantics and
// derives the corresponding permutation of selected interval columns.
func (r *IntervalSieve) Rename(cycle []int) (*IntervalSieve, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	sources, err := sparseIntervalOuterSources(len(r.sorts), cycle)
	if err != nil {
		return nil, err
	}
	oldInner := make(map[int]int, len(r.innerToSig))
	for local, outer := range r.innerToSig {
		oldInner[outer] = local
	}
	sorts := make([]ast.ASTHandle, len(r.sorts))
	mask := make([]bool, len(r.sorts))
	desired := make([]int, 0, len(r.innerToSig))
	for destination, source := range sources {
		sorts[destination] = r.sorts[source]
		if local, selected := oldInner[source]; selected {
			mask[destination] = true
			desired = append(desired, local)
		}
	}
	inner, err := sparseIntervalReorderInterval(r.inner, desired)
	if err != nil {
		return nil, err
	}
	return newIntervalSieve(r.manager, sorts, mask, inner)
}
