package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp rename_fn, specialized to
// finite-column cycles with unchanged explanation-column order.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// RenameFinite permutes finite columns while preserving each explanation row.
func (p *FiniteExplanationProduct) RenameFinite(cycle []int) (_ *FiniteExplanationProduct, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	permutation, err := tableRelationCyclePermutation(len(p.finiteSorts), cycle)
	if err != nil {
		return nil, err
	}
	sorts := make([]ast.ASTHandle, len(permutation))
	for i, from := range permutation {
		sorts[i] = p.finiteSorts[from]
	}
	result, err := p.newProduct(sorts, p.explanationColumns)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	facts, err := p.table.Facts()
	if err != nil {
		return nil, err
	}
	for _, fact := range facts {
		idx := uint64(fact[len(fact)-1])
		if idx >= uint64(len(p.inner)) || p.inner[idx] == nil {
			return nil, ErrTableRelation
		}
		row, e := p.inner[idx].Data()
		if e != nil {
			return nil, e
		}
		if len(row) != p.explanationColumns {
			return nil, ErrTableRelation
		}
		key := make(Fact, len(permutation)+1)
		for i, from := range permutation {
			key[i] = fact[from]
		}
		if e = result.addEncodedRow(key, row); e != nil {
			return nil, e
		}
	}
	return result, nil
}

// RenameExplanation permutes explanation columns while preserving finite keys.
func (p *FiniteExplanationProduct) RenameExplanation(cycle []int) (_ *FiniteExplanationProduct, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	permutation, err := tableRelationCyclePermutation(p.explanationColumns, cycle)
	if err != nil {
		return nil, err
	}
	result, err := p.newProduct(p.finiteSorts, p.explanationColumns)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	facts, err := p.table.Facts()
	if err != nil {
		return nil, err
	}
	for _, fact := range facts {
		idx := uint64(fact[len(fact)-1])
		if idx >= uint64(len(p.inner)) || p.inner[idx] == nil {
			return nil, ErrTableRelation
		}
		row, e := p.inner[idx].Data()
		if e != nil {
			return nil, e
		}
		if len(row) != p.explanationColumns {
			return nil, ErrTableRelation
		}
		renamed := make([]ast.ASTHandle, len(row))
		for i, from := range permutation {
			renamed[i] = row[from]
		}
		key := append(Fact(nil), fact...)
		if e = result.addEncodedRow(key, renamed); e != nil {
			return nil, e
		}
	}
	return result, nil
}
