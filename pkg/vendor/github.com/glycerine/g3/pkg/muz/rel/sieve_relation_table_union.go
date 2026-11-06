package rel

// Source: src/muz/rel/dl_sieve_relation.cpp
// sieve_relation_plugin::union_fn and mk_union_fn, instantiated for the
// retained concrete TableRelation specialization.

import "fmt"

// UnionFromTableRelation adds an ordinary finite table to a sieve target.
// Source permits this mixed union only when the sieve has no full columns.
// Delta, when present, must have the same all-inner sieve layout.
func (r *SieveRelation) UnionFromTableRelation(source *TableRelation, delta *SieveRelation) error {
	if err := r.open(); err != nil {
		return err
	}
	if source == nil || source.open() != nil || !r.NoSievedColumns() {
		return ErrSieve
	}
	sourceSignature, err := source.table.Signature()
	if err != nil {
		return err
	}
	if !r.signature.equal(sourceSignature) {
		return fmt.Errorf("%w: mixed union signatures differ", ErrSieve)
	}
	var innerDelta *HashTable
	if delta != nil {
		if delta.open() != nil || !delta.NoSievedColumns() || !r.sameLayout(delta) {
			return fmt.Errorf("%w: mixed union delta layout differs", ErrSieve)
		}
		innerDelta = delta.inner
	}
	if err = r.inner.Union(source.table, innerDelta); err != nil {
		return fmt.Errorf("%w: %v", ErrSieve, err)
	}
	return nil
}

// UnionIntoTableRelation adds this sieve source to an ordinary finite target.
// The sieve must have no full columns, so its inner table has the target's
// complete signature.
func (r *SieveRelation) UnionIntoTableRelation(target, delta *TableRelation) error {
	if err := r.open(); err != nil {
		return err
	}
	if target == nil || target.open() != nil || !r.NoSievedColumns() {
		return ErrSieve
	}
	targetSignature, err := target.table.Signature()
	if err != nil {
		return err
	}
	if !r.signature.equal(targetSignature) {
		return fmt.Errorf("%w: mixed union signatures differ", ErrSieve)
	}
	var innerDelta *HashTable
	if delta != nil {
		if delta.open() != nil {
			return ErrSieve
		}
		deltaSignature, cause := delta.table.Signature()
		if cause != nil {
			return cause
		}
		if !targetSignature.equal(deltaSignature) {
			return fmt.Errorf("%w: mixed union delta signature differs", ErrSieve)
		}
		innerDelta = delta.table
	}
	if err = target.table.Union(r.inner, innerDelta); err != nil {
		return fmt.Errorf("%w: %v", ErrSieve, err)
	}
	return nil
}
