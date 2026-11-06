package rel

// Source: src/muz/rel/dl_sieve_relation.cpp sieve_relation_plugin::union_fn,
// mixed sieve/ordinary cases with a delta using the opposite representation.

import "fmt"

// UnionFromTableRelationWithTableDelta unions an ordinary source into an
// all-inner sieve target while recording new rows in an ordinary delta.
func (r *SieveRelation) UnionFromTableRelationWithTableDelta(source, delta *TableRelation) error {
	if err := r.open(); err != nil {
		return err
	}
	if source == nil || source.open() != nil || delta == nil || delta.open() != nil || !r.NoSievedColumns() {
		return ErrSieve
	}
	sourceSignature, err := source.table.Signature()
	if err != nil {
		return err
	}
	deltaSignature, err := delta.table.Signature()
	if err != nil {
		return err
	}
	if !r.signature.equal(sourceSignature) || !r.signature.equal(deltaSignature) {
		return fmt.Errorf("%w: mixed union signatures differ", ErrSieve)
	}
	return r.inner.Union(source.table, delta.table)
}

// UnionIntoTableRelationWithSieveDelta unions an all-inner sieve source into
// an ordinary target while recording new rows in an all-inner sieve delta.
func (r *SieveRelation) UnionIntoTableRelationWithSieveDelta(target *TableRelation, delta *SieveRelation) error {
	if err := r.open(); err != nil {
		return err
	}
	if target == nil || target.open() != nil || delta == nil || delta.open() != nil || !r.NoSievedColumns() || !delta.NoSievedColumns() {
		return ErrSieve
	}
	targetSignature, err := target.table.Signature()
	if err != nil {
		return err
	}
	if !r.signature.equal(targetSignature) || !r.sameLayout(delta) {
		return fmt.Errorf("%w: mixed union signatures or delta layout differ", ErrSieve)
	}
	return target.table.Union(r.inner, delta.inner)
}
