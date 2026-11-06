package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

// Source sets a requested plugin before creating the relation. Go creates an
// empty relation eagerly and historically accepts prequery changes after fact
// insertion, so preserve those tuples during this representation migration.
func (e *PropositionalEngine) selectFiniteRepresentation(declaration ast.ASTHandle, kind string) error {
	state := e.predicates[declaration]
	if state == nil || state.table == nil {
		return ErrPropositionalHorn
	}
	if kind == "tr_lazy_sparse" {
		if e.lazyFinite != nil && e.lazyFinite.Owns(state.table) {
			return nil
		}
		if e.lazyFinite == nil {
			e.lazyFinite = rel.NewLazySparseFiniteFactory()
		}
		replacement, err := e.lazyFinite.Migrate(state.table)
		if err != nil {
			return err
		}
		return e.installDatalogTable(declaration, replacement)
	}
	switch kind {
	case "tr_sparse":
		if _, ok := state.table.(*rel.SparseTable); ok {
			return nil
		}
	case "tr_hashtable":
		if _, ok := state.table.(*rel.HashTable); ok {
			return nil
		}
	default:
		return ErrPropositionalHorn
	}
	signature, err := state.table.Signature()
	if err != nil {
		return err
	}
	var replacement rel.FiniteTable
	if kind == "tr_sparse" {
		replacement, err = rel.NewSparseTable(signature, 0)
	} else {
		replacement = rel.NewHashTable(signature)
	}
	if err != nil {
		return err
	}
	facts, err := state.table.Facts()
	if err != nil {
		return errors.Join(err, replacement.Close())
	}
	for _, fact := range facts {
		if _, err = replacement.Add(fact); err != nil {
			return errors.Join(err, replacement.Close())
		}
	}
	return e.installDatalogTable(declaration, replacement)
}
