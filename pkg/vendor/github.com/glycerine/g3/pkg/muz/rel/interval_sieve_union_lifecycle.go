package rel

// Source: src/muz/rel/dl_sieve_relation.cpp sieve_relation::reset and
// sieve_relation_plugin::union_fn, for the live IntervalRelation inner kind.

import "github.com/glycerine/g3/pkg/ast"

func (r *IntervalSieve) sameLayout(other *IntervalSieve) bool {
	if r == nil || other == nil || r.open() != nil || other.open() != nil || r.manager != other.manager || len(r.sorts) != len(other.sorts) || len(r.innerCols) != len(other.innerCols) {
		return false
	}
	for i := range r.sorts {
		if r.sorts[i] != other.sorts[i] || r.innerCols[i] != other.innerCols[i] {
			return false
		}
	}
	return true
}

// Reset empties the represented inner interval while preserving the outer
// signature and selected-column mapping.
func (r *IntervalSieve) Reset() error {
	if err := r.open(); err != nil {
		return err
	}
	selected := uint32(0)
	for _, inner := range r.innerCols {
		if inner {
			selected++
		}
	}
	r.inner = NewIntervalEmpty(selected)
	return nil
}

// UnionFrom forwards to the selected interval component. The source sieve
// plugin has no widen override, so relation-manager widening falls back to
// this ordinary union path and must not widen the inner interval relation.
// A non-nil delta has the identical sieve layout.
func (r *IntervalSieve) UnionFrom(source, delta *IntervalSieve, _ bool) (bool, error) {
	if !r.sameLayout(source) || delta != nil && !r.sameLayout(delta) {
		return false, ast.ErrASTDomain
	}
	if delta != nil {
		return r.inner.UnionWithDelta(source.inner, delta.inner, false)
	}
	return r.inner.Union(source.inner, false)
}
