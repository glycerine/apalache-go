package rel

// Source: src/muz/rel/dl_mk_simple_joins.cpp estimate_size and
// src/muz/rel/dl_base.h relation_base::get_size_estimate_rows.

import (
	"math"

	"github.com/glycerine/g3/pkg/ast"
)

type relationRowEstimator interface {
	SizeEstimateRows() (uint32, error)
}

// RelationSizeEstimateRows returns the source relation_base row estimate for
// the predicate's actual stored representation without creating a relation.
func (m *RelationManager) RelationSizeEstimateRows(predicate ast.ASTHandle) (uint32, bool, error) {
	if err := m.open(); err != nil {
		return 0, false, err
	}
	if product := m.explanations[predicate]; product != nil {
		rows, err := product.SizeEstimateRows()
		return rows, true, err
	}
	if product := m.relationLevel[predicate]; product != nil {
		if err := product.open(); err != nil {
			return 0, false, err
		}
		// The source whole-relation explanation is a product_relation. It
		// inherits relation_base's conservative estimate.
		return math.MaxUint32, true, nil
	}
	var table FiniteTable = m.finite[predicate]
	if table == nil {
		table = m.relations[predicate]
	}
	if table == nil {
		return 0, false, nil
	}
	if estimator, ok := table.(relationRowEstimator); ok {
		rows, err := estimator.SizeEstimateRows()
		return rows, true, err
	}
	rows, err := table.Len()
	if err != nil {
		return 0, true, err
	}
	if uint64(rows) > uint64(math.MaxUint32) {
		return math.MaxUint32, true, nil
	}
	return uint32(rows), true, nil
}
