package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp interpreted filtering and
// projection of inner columns, using the explanation assignment functor.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// AssignExplanation applies a rule-sort equality to each inner explanation
// row. The condition uses explanation-column variable indices.
func (p *FiniteExplanationProduct) AssignExplanation(condition ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	m := p.manager.manager
	family, op, left, right, binary, err := intervalParts(m, condition)
	if err != nil {
		return err
	}
	if !binary || family != m.BasicFamilyID() || op != ast.ASTOpEq {
		return ast.ErrASTDomain
	}
	rightKind, err := m.GetKind(right)
	if err != nil {
		return err
	}
	if rightKind == ast.ASTVar {
		left, right = right, left
	}
	leftKind, err := m.GetKind(left)
	if err != nil {
		return err
	}
	rightKind, err = m.GetKind(right)
	if err != nil {
		return err
	}
	if leftKind != ast.ASTVar || rightKind != ast.ASTApp {
		return ast.ErrASTDomain
	}
	sort, err := m.GetSort(left)
	if err != nil {
		return err
	}
	isRule, err := m.IsDatalogRuleSort(sort)
	if err != nil {
		return err
	}
	if !isRule {
		return ast.ErrASTDomain
	}
	facts, err := p.table.Facts()
	if err != nil {
		return err
	}
	for _, fact := range facts {
		idx := uint64(fact[len(fact)-1])
		if idx >= uint64(len(p.inner)) || p.inner[idx] == nil {
			return ErrTableRelation
		}
		matched, e := p.inner[idx].FilterInterpreted(condition)
		if e != nil {
			return e
		}
		if !matched {
			return ast.ErrASTDomain
		}
	}
	return nil
}

// ProjectExplanation removes explanation columns, retaining finite keys.
func (p *FiniteExplanationProduct) ProjectExplanation(removed []int) (_ *FiniteExplanationProduct, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	flags, err := tableRelationRemovedColumns(p.explanationColumns, removed)
	if err != nil {
		return nil, err
	}
	result, err := p.newProduct(p.finiteSorts, p.explanationColumns-len(removed))
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
		data := make([]ast.ASTHandle, 0, result.explanationColumns)
		for i, value := range row {
			if !flags[i] {
				data = append(data, value)
			}
		}
		key := append(Fact(nil), fact...)
		if e = result.addEncodedRow(key, data); e != nil {
			return nil, e
		}
	}
	return result, nil
}
