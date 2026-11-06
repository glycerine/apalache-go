package rel

// Source: src/muz/rel/dl_product_relation.cpp filter_negation_fn and
// src/muz/rel/dl_sieve_relation.cpp negation dispatch. The finite sieve is
// filtered independently; the whole-relation proof row is unchanged.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// FilterByNegation removes target rows matching negative. At the typed
// product boundary the actual AST sorts are available, so a plain explicitly
// selected negative relation can be safely lifted into a FormulaCheckedTable
// without weakening the generic finite-table sort guard.
func (p *RelationExplanationProduct) FilterByNegation(negative *RelationExplanationProduct, targetColumns, negativeColumns []int) (resultErr error) {
	if err := p.open(); err != nil {
		return err
	}
	if err := negative.open(); err != nil {
		return err
	}
	if p.manager != negative.manager || len(targetColumns) != len(negativeColumns) {
		return ast.ErrASTDomain
	}
	if p.numeric != nil || negative.numeric != nil {
		// The source bound, interval, and Karr plugins do not register
		// mk_filter_by_negation_fn; relation_manager returns no operation.
		return ErrCompilerUnsupported
	}
	if p.doc != nil || negative.doc != nil {
		if p.doc == nil || negative.doc == nil {
			return ErrUDocTableProduct
		}
		left, right := make([]uint32, len(targetColumns)), make([]uint32, len(negativeColumns))
		for i, column := range targetColumns {
			other := negativeColumns[i]
			if column < 0 || column >= len(p.finiteSorts) || other < 0 || other >= len(negative.finiteSorts) || p.finiteSorts[column] != negative.finiteSorts[other] {
				return ast.ErrASTDomain
			}
			left[i], right[i] = uint32(column), uint32(other)
		}
		return p.doc.Doc().FilterByNegation(negative.doc.Doc(), left, right)
	}
	for index, targetColumn := range targetColumns {
		negativeColumn := negativeColumns[index]
		if targetColumn < 0 || targetColumn >= len(p.finiteSorts) || negativeColumn < 0 || negativeColumn >= len(negative.finiteSorts) || p.finiteSorts[targetColumn] != negative.finiteSorts[negativeColumn] {
			return ast.ErrASTDomain
		}
	}
	negativeTable := negative.table
	if checked, ok := p.table.(*FormulaCheckedTable); ok {
		lifted, err := liftFormulaCheckedWithSorts(checked, negative.table, negative.finiteSorts)
		if err != nil {
			return err
		}
		if lifted != negative.table {
			defer func() { resultErr = errors.Join(resultErr, lifted.Close()) }()
		}
		negativeTable = lifted
	} else if checked, ok := p.table.(*FormulaCheckedFinitePair); ok {
		lifted, err := liftFormulaCheckedFinitePairWithSorts(checked, negative.table, negative.finiteSorts)
		if err != nil {
			return err
		}
		if lifted != negative.table {
			defer func() { resultErr = errors.Join(resultErr, lifted.Close()) }()
		}
		negativeTable = lifted
	}
	return finiteTableFilterNegation(p.table, negativeTable, targetColumns, negativeColumns)
}
