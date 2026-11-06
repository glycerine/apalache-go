package rel

import "github.com/glycerine/g3/pkg/ast"

// NewEmptyUDocTableProduct constructs the concrete product relation used when
// a source predicate inherits the DOC/table product plugin.
func NewEmptyUDocTableProduct(manager *RelationManager, factory *UDocRelationFactory, sorts []ast.ASTHandle) (*UDocTableProduct, error) {
	if manager == nil || manager.open() != nil || factory == nil || factory.open() != nil || factory.manager != manager.manager {
		return nil, ErrUDocTableProduct
	}
	signature, err := tableRelationSignature(manager, sorts)
	if err != nil {
		return nil, err
	}
	a := newUDocTableProductAttempt(manager)
	a.product.doc, err = NewEmptyUDocSieve(factory, sorts)
	if err == nil {
		a.product.table, err = manager.NewDefaultFiniteTable(signature)
	}
	return a.finish(err)
}
