package rel

// Source: src/muz/rel/dl_product_relation.cpp join_fn and
// src/muz/rel/dl_sieve_relation.cpp join_fn, specialized to the two concrete
// relation-level explanation sieves installed by dl_mk_explanations.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

// Join joins the finite sieves on finite columns and independently
// concatenates the whole-relation explanation rows. Intermediate results may
// therefore contain more than one explanation column; the later source
// assignment filter reduces those columns to the rule proof published for the
// result predicate.
func (p *RelationExplanationProduct) Join(other *RelationExplanationProduct, left, right []int) (_ *RelationExplanationProduct, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if err := other.open(); err != nil {
		return nil, err
	}
	if p.manager != other.manager || len(left) != len(right) {
		return nil, ast.ErrASTDomain
	}
	if p.numeric != nil || other.numeric != nil {
		if p.numeric == nil || other.numeric == nil {
			return nil, ErrTableRelation
		}
		leftColumns, rightColumns := make([]uint32, len(left)), make([]uint32, len(right))
		for index := range left {
			leftColumns[index], rightColumns[index] = uint32(left[index]), uint32(right[index])
		}
		numeric, err := p.numeric.Join(other.numeric, leftColumns, rightColumns)
		if err != nil {
			return nil, err
		}
		explanation, err := p.explanation.Join(other.explanation)
		if err != nil {
			return nil, errors.Join(err, numeric.Close())
		}
		return &RelationExplanationProduct{manager: p.manager, finiteSorts: append(append([]ast.ASTHandle(nil), p.finiteSorts...), other.finiteSorts...), numeric: numeric, explanation: explanation}, nil
	}
	for index, leftColumn := range left {
		rightColumn := right[index]
		if leftColumn < 0 || leftColumn >= len(p.finiteSorts) || rightColumn < 0 || rightColumn >= len(other.finiteSorts) || p.finiteSorts[leftColumn] != other.finiteSorts[rightColumn] {
			return nil, ast.ErrASTDomain
		}
	}
	if p.doc != nil || other.doc != nil {
		var doc *UDocTableProduct
		var err error
		leftColumns, rightColumns := make([]uint32, len(left)), make([]uint32, len(right))
		for i := range left {
			leftColumns[i], rightColumns[i] = uint32(left[i]), uint32(right[i])
		}
		switch {
		case p.doc != nil && other.doc != nil:
			doc, err = p.doc.Join(other.doc, leftColumns, rightColumns)
			if err != nil {
				return nil, errors.Join(errors.New("join two UDoc relation explanation products"), err)
			}
		case p.doc != nil:
			doc, err = JoinProductFinite(p.doc, other.finiteSorts, other.table, leftColumns, rightColumns)
			if err != nil {
				return nil, fmt.Errorf("join UDoc relation explanation product with finite relation (%T, sorts=%v, columns=%v/%v): %w", other.table, other.finiteSorts, left, right, err)
			}
		default:
			doc, err = JoinFiniteProduct(p.manager, p.finiteSorts, p.table, other.doc, leftColumns, rightColumns)
			if err != nil {
				return nil, errors.Join(errors.New("join finite relation with UDoc relation explanation product"), err)
			}
		}
		if err != nil {
			return nil, err
		}
		explanation, err := p.explanation.Join(other.explanation)
		if err != nil {
			return nil, errors.Join(err, doc.Close())
		}
		return &RelationExplanationProduct{
			manager:     p.manager,
			finiteSorts: append(append([]ast.ASTHandle(nil), p.finiteSorts...), other.finiteSorts...),
			doc:         doc,
			explanation: explanation,
		}, nil
	}
	first, second := p.table, other.table
	var firstOwned, secondOwned bool
	var err error
	if checked, ok := p.table.(*FormulaCheckedFinitePair); ok {
		second, err = liftFormulaCheckedFinitePairWithSorts(checked, other.table, other.finiteSorts)
		secondOwned = second != other.table
	} else if checked, ok := other.table.(*FormulaCheckedFinitePair); ok {
		first, err = liftFormulaCheckedFinitePairWithSorts(checked, p.table, p.finiteSorts)
		firstOwned = first != p.table
	} else if checked, ok := p.table.(*FormulaCheckedTable); ok {
		second, err = liftFormulaCheckedWithSorts(checked, other.table, other.finiteSorts)
		secondOwned = second != other.table
	} else if checked, ok := other.table.(*FormulaCheckedTable); ok {
		first, err = liftFormulaCheckedWithSorts(checked, p.table, p.finiteSorts)
		firstOwned = first != p.table
	}
	if err != nil {
		return nil, err
	}
	table, err := finiteTableJoin(first, second, left, right)
	if firstOwned {
		err = errors.Join(err, first.Close())
	}
	if secondOwned {
		err = errors.Join(err, second.Close())
	}
	if err != nil {
		if table != nil {
			err = errors.Join(err, table.Close())
		}
		return nil, err
	}
	explanation, err := p.explanation.Join(other.explanation)
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	return &RelationExplanationProduct{
		manager:     p.manager,
		finiteSorts: append(append([]ast.ASTHandle(nil), p.finiteSorts...), other.finiteSorts...),
		table:       table,
		explanation: explanation,
	}, nil
}
