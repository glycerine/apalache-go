package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp join_fn and
// src/muz/rel/dl_mk_explanations.cpp explanation_relation_plugin::join_fn.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// Join matches the specified finite columns and concatenates both finite
// tuples and their explanation rows. Explanation columns are not join keys.
func (p *FiniteExplanationProduct) Join(other *FiniteExplanationProduct, left, right []int) (_ *FiniteExplanationProduct, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if err := other.open(); err != nil {
		return nil, err
	}
	if p.manager != other.manager || len(left) != len(right) {
		return nil, ast.ErrASTDomain
	}
	for i, l := range left {
		r := right[i]
		if l < 0 || l >= len(p.finiteSorts) || r < 0 || r >= len(other.finiteSorts) || p.finiteSorts[l] != other.finiteSorts[r] {
			return nil, ast.ErrASTDomain
		}
	}
	sorts := append(append([]ast.ASTHandle(nil), p.finiteSorts...), other.finiteSorts...)
	template := p
	if _, checked := p.table.(*CheckedExplanationIndex); !checked {
		if _, otherChecked := other.table.(*CheckedExplanationIndex); otherChecked {
			template = other
		}
	}
	result, err := template.newProduct(sorts, p.explanationColumns+other.explanationColumns)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	a, err := p.table.Facts()
	if err != nil {
		return nil, err
	}
	b, err := other.table.Facts()
	if err != nil {
		return nil, err
	}
	for _, x := range a {
		xi := uint64(x[len(x)-1])
		if xi >= uint64(len(p.inner)) || p.inner[xi] == nil {
			return nil, ErrTableRelation
		}
		xrow, e := p.inner[xi].Data()
		if e != nil {
			return nil, e
		}
		if len(xrow) != p.explanationColumns {
			return nil, ErrTableRelation
		}
		for _, y := range b {
			match := true
			for i, l := range left {
				if x[l] != y[right[i]] {
					match = false
					break
				}
			}
			if !match {
				continue
			}
			yi := uint64(y[len(y)-1])
			if yi >= uint64(len(other.inner)) || other.inner[yi] == nil {
				return nil, ErrTableRelation
			}
			yrow, e := other.inner[yi].Data()
			if e != nil {
				return nil, e
			}
			if len(yrow) != other.explanationColumns {
				return nil, ErrTableRelation
			}
			key := make(Fact, 0, len(sorts)+1)
			key = append(key, x[:len(x)-1]...)
			key = append(key, y[:len(y)-1]...)
			key = append(key, 0)
			row := append(append([]ast.ASTHandle(nil), xrow...), yrow...)
			if e = result.addEncodedRow(key, row); e != nil {
				return nil, e
			}
		}
	}
	return result, nil
}
