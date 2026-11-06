package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// ToFormula returns an owning Boolean formula that enumerates the product's
// finite rows in relation-column order.
func (p *FiniteSparseProduct) ToFormula() (result ast.ASTRef, resultErr error) {
	if err := p.open(); err != nil {
		return result, err
	}
	flat, err := p.flatten()
	if err != nil {
		return result, err
	}
	defer flat.Close()
	facts, err := flat.Facts()
	if err != nil {
		return result, err
	}
	m := p.manager.manager
	var roots []ast.ASTRef
	ownedResult := false
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&roots[i]))
		}
		if resultErr != nil && ownedResult {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	hold := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := m.NewRef(value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		roots = append(roots, ref)
		return value, nil
	}
	disjunction := make([]ast.ASTHandle, 0, len(facts))
	for _, fact := range facts {
		conjuncts := make([]ast.ASTHandle, 0, len(fact))
		for column, value := range fact {
			variable, err := hold(m.MkVar(uint32(column), p.sorts[column]))
			if err != nil {
				return result, err
			}
			numeral, err := hold(m.MkDatalogNumeral(uint64(value), p.sorts[column]))
			if err != nil {
				return result, err
			}
			equality, err := hold(m.MkEq(variable, numeral))
			if err != nil {
				return result, err
			}
			conjuncts = append(conjuncts, equality)
		}
		conjunction, err := hold(m.MkAndArgs(conjuncts))
		if err != nil {
			return result, err
		}
		disjunction = append(disjunction, conjunction)
	}
	result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	ownedResult = true
	resultErr = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpOr, disjunction, result)
	return result, resultErr
}
