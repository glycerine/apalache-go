package rel

import "errors"

// storeEncodedInner publishes an already-owned inner relation beneath a fresh
// outer key. It consumes inner only on success. Source finite-product join's
// functional map retains this row even when the joined inner relation is
// empty; later garbage collection removes it.
func (p *FiniteSparseProduct) storeEncodedInner(key Fact, inner *SparseTable) error {
	if err := p.open(); err != nil {
		return err
	}
	if inner == nil || inner.open() != nil || !inner.signature.equal(p.innerSignature) || len(key) != len(p.tableColumns)+1 {
		return ErrTableRelation
	}
	probe := cloneFact(key)
	found, err := p.fetchInnerKey(probe)
	if err != nil {
		return err
	}
	if found {
		return ErrTableRelation
	}
	index, err := p.reserveIndex()
	if err != nil {
		return err
	}
	key = cloneFact(key)
	key[len(key)-1] = Element(index)
	if err = p.table.EnsureFact(key); err != nil {
		p.recycleReservedIndex(index)
		return err
	}
	if p.inner[index] != nil {
		_, removeErr := p.table.Remove(key)
		p.recycleReservedIndex(index)
		return errors.Join(ErrTableRelation, removeErr)
	}
	p.inner[index] = inner
	return nil
}
