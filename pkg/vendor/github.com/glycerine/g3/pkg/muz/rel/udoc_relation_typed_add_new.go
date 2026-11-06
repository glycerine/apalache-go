package rel

// Source: udoc_relation::add_new_fact and check_relation::add_new_fact.

import "github.com/glycerine/g3/pkg/ast"

// AddNewFact appends a tuple already established absent by the caller. Unlike
// AddFact, this source operation does not normalize the DOC union on insertion.
func (r *TypedUDocRelation) AddNewFact(values []ast.ASTHandle) error {
	fact, err := r.fact(values)
	if err != nil {
		return err
	}
	document, err := r.relation.factToDoc(fact)
	if err != nil {
		return err
	}
	if err = r.relation.elems.PushOwned(document); err != nil {
		return err
	}
	return r.checkerAddFact(values)
}
