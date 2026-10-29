package sat

// Source: src/sat/sat_simplifier.cpp:451–458; src/sat/sat_clause.h tmp_clause.

// One concrete borrowed dummy owner is part of the actual simplifier. This
// boundary routes only that explicit owner; it cannot discover foreign refs.
func (k *satSimplifierKernel) bindDummy(d *satTmpClause) error {
	if e := k.open(); e != nil {
		return e
	}
	if e := d.open(); e != nil {
		return e
	}
	if d.budget != k.budget {
		return ErrSATOwner
	}
	k.dummy = d.owner
	return nil
}
func (k *satSimplifierKernel) clauseReader(c StoredClauseRef) (*ClauseAllocator, error) {
	if c.owner == k.allocator.origin {
		return k.allocator, nil
	}
	if k.dummy != nil && c.owner == k.dummy.origin {
		return k.dummy, nil
	}
	return nil, ErrSATOwner
}
func (k *satSimplifierKernel) clauseSize(c StoredClauseRef) (uint32, error) {
	a, e := k.clauseReader(c)
	if e != nil {
		return 0, e
	}
	return a.Size(c)
}
func (k *satSimplifierKernel) clauseLiteral(c StoredClauseRef, i uint32) (Literal, error) {
	a, e := k.clauseReader(c)
	if e != nil {
		return 0, e
	}
	return a.Literal(c, i)
}
func (k *satSimplifierKernel) clauseApprox(c StoredClauseRef) (satApproxSet, error) {
	a, e := k.clauseReader(c)
	if e != nil {
		return 0, e
	}
	return a.Approx(c)
}
func (k *satSimplifierKernel) clauseRemoved(c StoredClauseRef) (bool, error) {
	a, e := k.clauseReader(c)
	if e != nil {
		return false, e
	}
	return a.Removed(c)
}
