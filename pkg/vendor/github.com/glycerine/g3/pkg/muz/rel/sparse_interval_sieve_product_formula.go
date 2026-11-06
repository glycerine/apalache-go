package rel

// Source: src/muz/rel/dl_product_relation.cpp product_relation::to_formula.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// ToFormula returns the conjunction of the independent sparse finite and
// interval-sieve component formulas in outer relation-column coordinates.
func (p *SparseIntervalSieveProduct) ToFormula() (result ast.ASTRef, resultErr error) {
	if err := p.open(); err != nil {
		return result, err
	}
	m := p.manager.manager
	facts, err := p.finite.Facts()
	if err != nil {
		return result, err
	}
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
	hold := func(handle ast.ASTHandle, cause error) (ast.ASTHandle, error) {
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		ref, cause := m.NewRef(handle)
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		roots = append(roots, ref)
		return handle, nil
	}
	rows := make([]ast.ASTHandle, 0, len(facts))
	for _, fact := range facts {
		if len(fact) != len(p.finiteColumns) {
			return result, ErrSparseIntervalProduct
		}
		equalities := make([]ast.ASTHandle, 0, len(fact))
		for local, value := range fact {
			outer := p.finiteColumns[local]
			variable, cause := hold(m.MkVar(uint32(outer), p.sorts[outer]))
			if cause != nil {
				return result, cause
			}
			valueRef, cause := p.manager.DecodeValue(p.finiteSorts[local], value)
			if cause != nil {
				return result, cause
			}
			roots = append(roots, valueRef)
			valueHandle, cause := m.RefGet(valueRef)
			if cause != nil {
				return result, cause
			}
			equality, cause := hold(m.MkEq(variable, valueHandle))
			if cause != nil {
				return result, cause
			}
			equalities = append(equalities, equality)
		}
		row, cause := hold(m.MkAndArgs(equalities))
		if cause != nil {
			return result, cause
		}
		rows = append(rows, row)
	}
	finite, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	roots = append(roots, finite)
	if err = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpOr, rows, finite); err != nil {
		return result, err
	}
	interval, err := p.interval.ToFormula()
	if err != nil {
		return result, err
	}
	roots = append(roots, interval)
	finiteHandle, err := m.RefGet(finite)
	if err != nil {
		return result, err
	}
	intervalHandle, err := m.RefGet(interval)
	if err != nil {
		return result, err
	}
	result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	ownedResult = true
	if err = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpAnd, []ast.ASTHandle{finiteHandle, intervalHandle}, result); err != nil {
		return result, err
	}
	return result, nil
}
