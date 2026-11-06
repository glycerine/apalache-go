package rel

// Source: src/muz/rel/dl_sieve_relation.cpp
// sieve_relation_plugin::mk_union_fn and union_fn::operator(), for unions
// whose only representation mismatch is the delta relation.

import "github.com/glycerine/g3/pkg/ast"

// UnionFromSieveWithIntervalDelta unions aligned sieve relations while writing
// the change into an ordinary interval delta. The raw delta makes this a mixed
// source operation, so every sieve column must be retained.
func (r *IntervalSieve) UnionFromSieveWithIntervalDelta(source *IntervalSieve, delta *IntervalRelation, deltaSorts []ast.ASTHandle, _ bool) (bool, error) {
	if !r.sameLayout(source) || !r.matchesRawInterval(delta, deltaSorts) {
		return false, ast.ErrASTDomain
	}
	return r.inner.UnionWithDelta(source.inner, delta, false)
}

// UnionIntervalsWithSieveDelta unions ordinary interval relations while
// writing the change into this sieve delta. The source dispatcher admits the
// sieve delta only when it retains every outer column.
func (r *IntervalSieve) UnionIntervalsWithSieveDelta(target, source *IntervalRelation, sorts []ast.ASTHandle, _ bool) (bool, error) {
	if !r.matchesRawInterval(target, sorts) || !r.matchesRawInterval(source, sorts) {
		return false, ast.ErrASTDomain
	}
	return target.UnionWithDelta(source, r.inner, false)
}
