package rel

// Source: src/muz/rel/dl_sieve_relation.cpp
// sieve_relation_plugin::mk_union_fn and union_fn::operator(), instantiated
// with interval_relation. Mixed raw/sieve unions are available only when the
// sieve retains every outer column.

import "github.com/glycerine/g3/pkg/ast"

func (r *IntervalSieve) matchesRawInterval(raw *IntervalRelation, rawSorts []ast.ASTHandle) bool {
	if r == nil || r.open() != nil || raw == nil || raw.values == nil || len(r.innerToSig) != len(r.sorts) || len(rawSorts) != len(r.sorts) {
		return false
	}
	for i := range r.sorts {
		if !r.innerCols[i] || r.sorts[i] != rawSorts[i] {
			return false
		}
	}
	arity, err := raw.Arity()
	return err == nil && int(arity) == len(rawSorts)
}

// UnionFromInterval applies the source mixed union with a sieve target and
// ordinary interval source/delta. A requested widening reaches the sieve
// plugin's ordinary-union fallback, so the inner operation never widens.
func (r *IntervalSieve) UnionFromInterval(source, delta *IntervalRelation, sourceSorts []ast.ASTHandle, _ bool) (bool, error) {
	if !r.matchesRawInterval(source, sourceSorts) || delta != nil && !r.matchesRawInterval(delta, sourceSorts) {
		return false, ast.ErrASTDomain
	}
	if delta != nil {
		return r.inner.UnionWithDelta(source, delta, false)
	}
	return r.inner.Union(source, false)
}

// UnionFromIntervalWithSieveDelta is the same mixed target/source operation
// when the caller's delta has the sieve representation.
func (r *IntervalSieve) UnionFromIntervalWithSieveDelta(source *IntervalRelation, sourceSorts []ast.ASTHandle, delta *IntervalSieve, _ bool) (bool, error) {
	if !r.matchesRawInterval(source, sourceSorts) || !r.sameLayout(delta) || len(r.innerToSig) != len(r.sorts) {
		return false, ast.ErrASTDomain
	}
	return r.inner.UnionWithDelta(source, delta.inner, false)
}

// UnionIntoInterval applies the symmetric source path with an ordinary
// interval target/delta and a sieve source.
func (r *IntervalSieve) UnionIntoInterval(target, delta *IntervalRelation, targetSorts []ast.ASTHandle, _ bool) (bool, error) {
	if !r.matchesRawInterval(target, targetSorts) || delta != nil && !r.matchesRawInterval(delta, targetSorts) {
		return false, ast.ErrASTDomain
	}
	if delta != nil {
		return target.UnionWithDelta(r.inner, delta, false)
	}
	return target.Union(r.inner, false)
}

// UnionIntoIntervalWithSieveDelta is the symmetric mixed operation when the
// caller's delta has the sieve representation.
func (r *IntervalSieve) UnionIntoIntervalWithSieveDelta(target *IntervalRelation, targetSorts []ast.ASTHandle, delta *IntervalSieve, _ bool) (bool, error) {
	if !r.matchesRawInterval(target, targetSorts) || !r.sameLayout(delta) || len(r.innerToSig) != len(r.sorts) {
		return false, ast.ErrASTDomain
	}
	return target.UnionWithDelta(r.inner, delta.inner, false)
}
