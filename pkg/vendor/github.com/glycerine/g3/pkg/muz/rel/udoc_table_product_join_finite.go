package rel

// Source: dl_product_relation.cpp join_fn::init, where a product's DOC
// component is paired with a full DOC relation and its table component is
// paired with an ordinary finite input.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

// JoinProductFinite joins a product input with a raw finite-table input. The
// product/table component is joined with the table; a full DOC component over
// the finite input signature supplies the unmatched side of the product.
func JoinProductFinite(product *UDocTableProduct, finiteSorts []ast.ASTHandle, finite FiniteTable, productColumns, finiteColumns []uint32) (*UDocTableProduct, error) {
	return joinProductFinite(product, finiteSorts, finite, productColumns, finiteColumns, true)
}

// JoinFiniteProduct preserves the opposite source operand and column order.
func JoinFiniteProduct(manager *RelationManager, finiteSorts []ast.ASTHandle, finite FiniteTable, product *UDocTableProduct, finiteColumns, productColumns []uint32) (*UDocTableProduct, error) {
	if product == nil || manager != product.manager {
		return nil, ErrUDocTableProduct
	}
	return joinProductFinite(product, finiteSorts, finite, productColumns, finiteColumns, false)
}

func joinProductFinite(product *UDocTableProduct, finiteSorts []ast.ASTHandle, finite FiniteTable, productColumns, finiteColumns []uint32, productFirst bool) (*UDocTableProduct, error) {
	if product == nil || product.open() != nil || finite == nil || rawUDocProductFiniteKind(finite) == 0 || len(productColumns) != len(finiteColumns) {
		return nil, ErrUDocTableProduct
	}
	if err := product.productJoinTableReady(); err != nil {
		return nil, fmt.Errorf("DOC/finite join source product: %w", err)
	}
	if lazy, ok := finite.(*LazySparseTable); ok && (product.manager.lazyFinite == nil || !product.manager.lazyFinite.Owns(lazy)) {
		return nil, ErrUDocTableProduct
	}
	signature, err := tableRelationSignature(product.manager, finiteSorts)
	if err != nil {
		return nil, fmt.Errorf("DOC/finite join finite signature: %w", err)
	}
	actual, err := finite.Signature()
	if err != nil || !signature.equal(actual) {
		return nil, fmt.Errorf("DOC/finite join input table signature (%v, expected %v): %w", actual, signature, ErrUDocTableProduct)
	}
	for i, column := range productColumns {
		other := finiteColumns[i]
		if column >= uint32(len(product.doc.sorts)) || other >= uint32(len(finiteSorts)) || product.doc.sorts[column] != finiteSorts[other] {
			return nil, ErrUDocTableProduct
		}
	}
	if len(finiteSorts) == 0 {
		// A nullary relation is the Cartesian identity when it contains its
		// unit tuple and the empty relation otherwise. Avoid constructing a DOC
		// sieve for this source Hash fallback signature.
		rows, err := finite.Len()
		if err != nil {
			return nil, err
		}
		if rows == 0 {
			return product.EmptyLike()
		}
		return product.Clone()
	}
	a := newUDocTableProductAttempt(product.manager)
	a.fullDoc, err = NewFullUDocSieve(product.doc.inner.factory, finiteSorts)
	if err != nil {
		return a.finish(fmt.Errorf("DOC/finite join full DOC sieve: %w", err))
	}
	if productFirst {
		a.product.doc, err = product.doc.Join(a.fullDoc, productColumns, finiteColumns)
		if err == nil {
			leftTable, leftMask := product.table, productTableMask(product.doc.sorts, product.tableColumns)
			if leftTable == nil {
				leftTable, leftMask, err = a.fullTableish(product.doc.sorts, finite)
				if err != nil {
					err = fmt.Errorf("DOC/finite join full left table: %w", err)
				}
			}
			if err == nil {
				a.product.table, a.product.tableColumns, err = joinProductTableComponents(product.manager, product.doc.sorts, leftTable, leftMask, finiteSorts, finite, productTableMask(finiteSorts, nil), productColumns, finiteColumns)
				if err != nil {
					err = fmt.Errorf("DOC/finite join table components: %w", err)
				}
			}
		}
	} else {
		a.product.doc, err = a.fullDoc.Join(product.doc, finiteColumns, productColumns)
		if err == nil {
			rightTable, rightMask := product.table, productTableMask(product.doc.sorts, product.tableColumns)
			if rightTable == nil {
				rightTable, rightMask, err = a.fullTableish(product.doc.sorts, finite)
				if err != nil {
					err = fmt.Errorf("finite/DOC join full right table: %w", err)
				}
			}
			if err == nil {
				a.product.table, a.product.tableColumns, err = joinProductTableComponents(product.manager, finiteSorts, finite, productTableMask(finiteSorts, nil), product.doc.sorts, rightTable, rightMask, finiteColumns, productColumns)
				if err != nil {
					err = fmt.Errorf("finite/DOC join table components: %w", err)
				}
			}
		}
	}
	return a.finish(err)
}
