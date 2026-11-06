package rel

// Source: udoc_plugin::negation_filter_fn in src/muz/rel/udoc_relation.cpp.

import (
	"github.com/glycerine/g3/pkg/ast"
)

// FilterByNegation removes tuples matching a negative relation on the given
// original typed columns, using the shared plugin's intermediate DOC manager.
func (r *TypedUDocRelation) FilterByNegation(negative *TypedUDocRelation, columns, negativeColumns []uint32) (resultErr error) {
	if err := r.open(); err != nil {
		return err
	}
	if negative == nil || negative.open() != nil || r.factory != negative.factory || len(columns) != len(negativeColumns) {
		return ErrDOC
	}
	if len(columns) == 0 && len(negative.sorts) != 0 {
		return ErrDOC
	}
	for index, column := range columns {
		other := negativeColumns[index]
		if uint64(column) >= uint64(len(r.sorts)) || uint64(other) >= uint64(len(negative.sorts)) || r.sorts[column] != negative.sorts[other] {
			return ast.ErrASTDomain
		}
	}
	if r.checker != nil {
		if negative.checker == nil {
			return ErrRelationVerification
		}
		expected, err := r.checker.negationFormulaRefs(r.checker.formula, negative.checker.formula, r.sorts, negative.sorts, columns, negativeColumns)
		if err != nil {
			return err
		}
		owner := &typedUDocFormulaOwner{manager: r.factory.manager, roots: []ast.ASTRef{expected}}
		defer func() {
			if resultErr == nil {
				formula, err := r.factory.manager.RefGet(expected)
				if err == nil {
					err = r.checkFormula(formula)
				}
				resultErr = err
			}
			resultErr = finishTypedUDocFormula(owner, resultErr)
		}()
	}
	direct := r.relation.manager == negative.relation.manager && len(columns) == len(r.sorts) && len(negativeColumns) == len(negative.sorts)
	seen := make(map[uint32]bool, len(columns))
	for index, column := range columns {
		if column != negativeColumns[index] || seen[column] {
			direct = false
		}
		seen[column] = true
	}
	empty, err := r.FastEmpty()
	if err != nil {
		return err
	}
	negativeEmpty, err := negative.FastEmpty()
	if err != nil {
		return err
	}
	if empty || negativeEmpty || len(negative.sorts) == 0 || direct {
		return r.relation.FilterByNegation(negative.relation, columns, negativeColumns, nil)
	}
	widths := append(append([]uint32(nil), r.relation.widths...), negative.relation.widths...)
	manager, err := r.factory.managerFor(widths)
	if err != nil {
		return err
	}
	return r.relation.FilterByNegation(negative.relation, columns, negativeColumns, manager)
}
