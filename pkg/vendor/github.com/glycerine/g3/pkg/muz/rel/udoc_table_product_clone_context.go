package rel

// Source: product_relation::clone within a scoped query copy. Go query
// contexts have separate finite codecs and lazy factories over the same AST
// manager, while borrowing the source DOC factory's width-indexed managers.

// CloneForManager clones both concrete components into a query context's
// codec. A lazy component is copied into that context's actual lazy factory;
// cloning its old root alone would keep an incompatible source engine.
// The shared DOC factory must outlive the returned owner.
func (p *UDocTableProduct) CloneForManager(manager *RelationManager) (*UDocTableProduct, error) {
	if p.open() != nil || manager == nil || manager.open() != nil || manager.manager != p.manager.manager {
		return nil, ErrUDocTableProduct
	}
	a := newUDocTableProductAttempt(manager)
	var err error
	a.product.doc, err = p.doc.Clone()
	if err != nil {
		return a.finish(err)
	}
	if p.table == nil {
		return a.finish(nil)
	}
	a.product.tableColumns = append([]bool(nil), p.tableColumns...)
	source, lazy := p.table.(*LazySparseTable)
	if !lazy {
		a.product.table, err = p.table.CloneFinite()
		return a.finish(err)
	}
	if source.open() != nil || manager.lazyFinite == nil || manager.lazyFinite.open() != nil {
		return a.finish(ErrUDocTableProduct)
	}
	if manager.lazyFinite.Owns(source) {
		a.product.table, err = source.CloneFinite()
		return a.finish(err)
	}
	signature, err := source.Signature()
	if err != nil {
		return a.finish(err)
	}
	var target *LazySparseTable
	if signature.Len() == 0 {
		// Projected compiler temporaries keep their concrete lazy root even
		// when its evaluated nullary storage uses the source Hash fallback.
		target = manager.lazyFinite.engine.adopt(&lazySparseNode{
			kind: LazyTableBase, signature: signature, nullary: NewHashTable(signature),
		})
	} else {
		target, err = manager.lazyFinite.engine.New(signature, source.root.functional)
	}
	if target != nil {
		a.product.table = target
	}
	if err != nil {
		return a.finish(err)
	}
	facts, err := source.Facts()
	if err != nil {
		return a.finish(err)
	}
	for _, fact := range facts {
		if _, err = target.Add(fact); err != nil {
			return a.finish(err)
		}
	}
	return a.finish(nil)
}
