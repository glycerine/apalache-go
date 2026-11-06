package rel

// Source: src/muz/rel/dl_product_relation.cpp transform_fn and
// src/muz/rel/dl_sieve_relation.cpp mk_project_fn. Projecting finite columns
// is an identity on the independent whole-relation explanation sieve.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

// Project removes finite columns and preserves an independent copy of the
// whole-relation explanation row.
func (p *RelationExplanationProduct) Project(removed []int) (_ *RelationExplanationProduct, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	flags, err := tableRelationRemovedColumns(len(p.finiteSorts), removed)
	if err != nil {
		return nil, err
	}
	return p.projectComponents(removed, nil, flags)
}

// ProjectColumns applies a projection to the normalized concrete product
// layout: every finite column first, followed by every whole-relation proof
// column. This is the layout emitted by the explanation rule compiler after
// source product/sieve joins. A projection may remove every proof column; the
// resulting nonempty zero-column ExplanationRelation is the source full
// identity used while duplicating a finite column.
func (p *RelationExplanationProduct) ProjectColumns(removed []int) (_ *RelationExplanationProduct, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	proofSorts, err := p.explanation.signature()
	if err != nil {
		return nil, err
	}
	if _, err = tableRelationRemovedColumns(len(p.finiteSorts)+len(proofSorts), removed); err != nil {
		return nil, err
	}
	finiteRemoved := make([]int, 0, len(removed))
	proofRemoved := make([]int, 0, len(removed))
	for _, column := range removed {
		if column < len(p.finiteSorts) {
			finiteRemoved = append(finiteRemoved, column)
		} else {
			proofRemoved = append(proofRemoved, column-len(p.finiteSorts))
		}
	}
	finiteFlags, err := tableRelationRemovedColumns(len(p.finiteSorts), finiteRemoved)
	if err != nil {
		return nil, err
	}
	return p.projectComponents(finiteRemoved, proofRemoved, finiteFlags)
}

func (p *RelationExplanationProduct) projectComponents(finiteRemoved, proofRemoved []int, finiteFlags []bool) (_ *RelationExplanationProduct, resultErr error) {
	var table FiniteTable
	var doc *UDocTableProduct
	var numeric *relationExplanationNumeric
	var err error
	if p.doc != nil {
		columns := make([]uint32, len(finiteRemoved))
		for i, column := range finiteRemoved {
			columns[i] = uint32(column)
		}
		doc, err = p.doc.Project(columns)
	} else if p.numeric != nil {
		columns := make([]uint32, len(finiteRemoved))
		for i, column := range finiteRemoved {
			columns[i] = uint32(column)
		}
		numeric, err = p.numeric.Project(columns)
	} else {
		table, err = finiteTableProject(p.table, finiteRemoved)
	}
	if err != nil {
		return nil, fmt.Errorf("finite projection: %w", err)
	}
	explanation, err := p.explanation.Project(proofRemoved)
	if err != nil {
		if doc != nil {
			return nil, errors.Join(fmt.Errorf("proof projection: %w", err), doc.Close())
		}
		if numeric != nil {
			return nil, errors.Join(fmt.Errorf("proof projection: %w", err), numeric.Close())
		}
		return nil, errors.Join(fmt.Errorf("proof projection: %w", err), table.Close())
	}
	sorts := make([]ast.ASTHandle, 0, len(p.finiteSorts)-len(finiteRemoved))
	for column, sort := range p.finiteSorts {
		if !finiteFlags[column] {
			sorts = append(sorts, sort)
		}
	}
	return &RelationExplanationProduct{manager: p.manager, finiteSorts: sorts, table: table, doc: doc, numeric: numeric, explanation: explanation}, nil
}
