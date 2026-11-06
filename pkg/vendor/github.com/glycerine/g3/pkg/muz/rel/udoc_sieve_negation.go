package rel

// Source: dl_sieve_relation.cpp mk_filter_by_negation_fn maps only equalities
// owned by both inner relations and applies the resulting filter to them.

import "github.com/glycerine/g3/pkg/ast"

func (r *UDocSieve) FilterByNegation(negative *UDocSieve, columns, negativeColumns []uint32) error {
	if r.open() != nil || negative == nil || negative.open() != nil || r.inner.factory != negative.inner.factory || len(columns) != len(negativeColumns) {
		return ErrSieve
	}
	var innerColumns, innerNegativeColumns []uint32
	for i, column := range columns {
		other := negativeColumns[i]
		if column >= uint32(len(r.sorts)) || other >= uint32(len(negative.sorts)) || r.sorts[column] != negative.sorts[other] {
			return ast.ErrASTDomain
		}
		targetInner, negativeInner := r.sigToInner[column] >= 0, negative.sigToInner[other] >= 0
		if !targetInner && negativeInner {
			// The target column is full while the negative side restricts it;
			// the source sieve keeps the target unchanged for soundness.
			return nil
		}
		if targetInner && negativeInner {
			innerColumns = append(innerColumns, uint32(r.sigToInner[column]))
			innerNegativeColumns = append(innerNegativeColumns, uint32(negative.sigToInner[other]))
		}
		// A full negative column makes its equality tautological, so the
		// source drops that comparison from the inner operation.
	}
	return r.inner.FilterByNegation(negative.inner, innerColumns, innerNegativeColumns)
}
