package rel

// Source: src/muz/rel/rel_context.cpp rel_context::has_facts. The source
// queries the predicate's actual relation_base regardless of its concrete
// table or explanation-product representation.

import "github.com/glycerine/g3/pkg/ast"

// HasFacts reports whether an already materialized predicate relation is
// nonempty without creating storage or changing its selected representation.
func (m *RelationManager) HasFacts(predicate ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	if product := m.explanations[predicate]; product != nil {
		rows, err := product.Len()
		return rows != 0, err
	}
	if product := m.relationLevel[predicate]; product != nil {
		rows, err := product.Len()
		return rows != 0, err
	}
	var table FiniteTable = m.finite[predicate]
	if table == nil {
		if hash := m.relations[predicate]; hash != nil {
			table = hash
		}
	}
	if table == nil {
		return false, nil
	}
	rows, err := table.Len()
	return rows != 0, err
}
