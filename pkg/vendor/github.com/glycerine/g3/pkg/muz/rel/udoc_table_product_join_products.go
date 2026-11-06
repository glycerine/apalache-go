package rel

// Source: dl_product_relation.cpp join_fn::init pairs matching underlying DOC
// kinds and pairs the actual table components. No missing component needs a
// full relation in this concrete product/product case.

// Join preserves source operand order, concatenating the two outer signatures
// and sieve masks. Inputs remain borrowed, including when both operands alias.
func (p *UDocTableProduct) Join(other *UDocTableProduct, left, right []uint32) (*UDocTableProduct, error) {
	if p.open() != nil || other.open() != nil || p.manager != other.manager || p.doc.inner.factory != other.doc.inner.factory || len(left) != len(right) {
		return nil, ErrUDocTableProduct
	}
	if err := p.productJoinTableReady(); err != nil {
		return nil, err
	}
	if err := other.productJoinTableReady(); err != nil {
		return nil, err
	}
	for i, column := range left {
		otherColumn := right[i]
		if column >= uint32(len(p.doc.sorts)) || otherColumn >= uint32(len(other.doc.sorts)) || p.doc.sorts[column] != other.doc.sorts[otherColumn] {
			return nil, ErrUDocTableProduct
		}
	}
	a := newUDocTableProductAttempt(p.manager)
	var err error
	a.product.doc, err = p.doc.Join(other.doc, left, right)
	if err != nil {
		return a.finish(err)
	}
	switch {
	case p.table == nil && other.table == nil:
	case p.table == nil:
		full, mask, cause := a.fullTableish(p.doc.sorts, other.table)
		if cause == nil {
			a.product.table, a.product.tableColumns, cause = joinProductTableComponents(p.manager, p.doc.sorts, full, mask, other.doc.sorts, other.table, productTableMask(other.doc.sorts, other.tableColumns), left, right)
		}
		err = cause
	case other.table == nil:
		full, mask, cause := a.fullTableish(other.doc.sorts, p.table)
		if cause == nil {
			a.product.table, a.product.tableColumns, cause = joinProductTableComponents(p.manager, p.doc.sorts, p.table, productTableMask(p.doc.sorts, p.tableColumns), other.doc.sorts, full, mask, left, right)
		}
		err = cause
	default:
		// dl_relation_manager.cpp's default table join selects the first
		// capable actual plugin. Mixed concrete kinds use that existing
		// fallback; the context's preferred table kind is not consulted.
		a.product.table, a.product.tableColumns, err = joinProductTableComponents(p.manager, p.doc.sorts, p.table, productTableMask(p.doc.sorts, p.tableColumns), other.doc.sorts, other.table, productTableMask(other.doc.sorts, other.tableColumns), left, right)
	}
	return a.finish(err)
}

func (p *UDocTableProduct) productJoinTableReady() error {
	if p.table != nil && rawUDocProductFiniteKind(p.table) == 0 {
		return ErrUDocTableProduct
	}
	if p.table == nil {
		return nil
	}
	// Functional table suffixes reorder joined columns. The DOC sieve's
	// ordinary relation signature has no such suffix in this live consumer.
	functional := 0
	var err error
	switch table := p.table.(type) {
	case *SparseTable:
		functional, err = table.FunctionalColumns()
	case *LazySparseTable:
		if p.manager.lazyFinite == nil || !p.manager.lazyFinite.Owns(table) {
			return ErrUDocTableProduct
		}
		functional, err = table.FunctionalColumns()
	}
	if err != nil {
		return err
	}
	if functional != 0 {
		return ErrUDocTableProduct
	}
	mask := productTableMask(p.doc.sorts, p.tableColumns)
	sorts := productTableInnerSorts(p.doc.sorts, mask)
	expected, err := tableRelationSignature(p.manager, sorts)
	if err != nil {
		return err
	}
	actual, err := p.table.Signature()
	if err != nil {
		return err
	}
	if !expected.equal(actual) {
		return ErrUDocTableProduct
	}
	return nil
}
