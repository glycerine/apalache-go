package rel

// Source: dl_product_relation.cpp join_fn::init matches the actual DOC kind
// through a sieve and supplies a full finite relation only for the unmatched
// table component. This is the live raw-DOC/product case, not product/product.

import "github.com/glycerine/g3/pkg/ast"

// JoinUDocRelations wraps a raw DOC join as a DOC-only product component.
func JoinUDocRelations(manager *RelationManager, left, right *TypedUDocRelation, leftColumns, rightColumns []uint32) (*UDocTableProduct, error) {
	if manager == nil || manager.open() != nil || left == nil || right == nil || left.open() != nil || right.open() != nil || left.factory != right.factory || left.factory.manager != manager.manager || left.FormulaChecked() || right.FormulaChecked() || len(leftColumns) != len(rightColumns) {
		return nil, ErrUDocTableProduct
	}
	for i, column := range leftColumns {
		if column >= uint32(len(left.sorts)) || rightColumns[i] >= uint32(len(right.sorts)) || left.sorts[column] != right.sorts[rightColumns[i]] {
			return nil, ErrUDocTableProduct
		}
	}
	a := newUDocTableProductAttempt(manager)
	inner, err := left.Join(right, leftColumns, rightColumns)
	if inner != nil {
		a.inners = append(a.inners, inner)
	}
	if err != nil {
		return a.finish(err)
	}
	sorts := append(append([]ast.ASTHandle(nil), left.sorts...), right.sorts...)
	mask := make([]bool, len(sorts))
	for i := range mask {
		mask[i] = true
	}
	a.product.doc, err = AdoptUDocSieve(sorts, mask, inner)
	if err == nil {
		a.inners = a.inners[:len(a.inners)-1]
	}
	return a.finish(err)
}

// JoinUDocProduct joins raw unary DOC with an existing two-component product.
// The product's borrowed manager and shared DOC factory must remain live.
func JoinUDocProduct(doc *TypedUDocRelation, product *UDocTableProduct, docColumns, productColumns []uint32) (*UDocTableProduct, error) {
	return joinUDocProduct(doc, product, docColumns, productColumns, true)
}

// JoinProductUDoc preserves the opposite source operand and column order.
func JoinProductUDoc(product *UDocTableProduct, doc *TypedUDocRelation, productColumns, docColumns []uint32) (*UDocTableProduct, error) {
	return joinUDocProduct(doc, product, docColumns, productColumns, false)
}

func joinUDocProduct(doc *TypedUDocRelation, product *UDocTableProduct, docColumns, productColumns []uint32, docFirst bool) (*UDocTableProduct, error) {
	if product.open() != nil || doc == nil || doc.open() != nil || doc.FormulaChecked() || doc.factory != product.doc.inner.factory || len(doc.sorts) != 1 || len(docColumns) != len(productColumns) || product.table != nil && rawUDocProductFiniteKind(product.table) == 0 {
		return nil, ErrUDocTableProduct
	}
	for i, column := range docColumns {
		other := productColumns[i]
		if column >= uint32(len(doc.sorts)) || other >= uint32(len(product.doc.sorts)) || doc.sorts[column] != product.doc.sorts[other] {
			return nil, ErrUDocTableProduct
		}
	}
	a := newUDocTableProductAttempt(product.manager)
	var err error
	if docFirst {
		a.product.doc, err = doc.JoinSieve(product.doc, docColumns, productColumns)
	} else {
		a.product.doc, err = product.doc.JoinUDoc(doc, productColumns, docColumns)
	}
	if err != nil {
		return a.finish(err)
	}
	if product.table == nil {
		return a.finish(nil)
	}
	full, fullMask, err := a.fullTableish(doc.sorts, product.table)
	if err != nil {
		return a.finish(err)
	}
	if docFirst {
		a.product.table, a.product.tableColumns, err = joinProductTableComponents(product.manager, doc.sorts, full, fullMask, product.doc.sorts, product.table, productTableMask(product.doc.sorts, product.tableColumns), docColumns, productColumns)
	} else {
		a.product.table, a.product.tableColumns, err = joinProductTableComponents(product.manager, product.doc.sorts, product.table, productTableMask(product.doc.sorts, product.tableColumns), doc.sorts, full, fullMask, productColumns, docColumns)
	}
	return a.finish(err)
}
