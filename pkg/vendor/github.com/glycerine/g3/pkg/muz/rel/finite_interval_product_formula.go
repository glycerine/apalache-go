package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp, to_formula.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// ToFormula returns an owning source-order formula for the product rows. The
// source uses leading signature positions for table values and shifts inner
// variables by the number of table columns, even for interleaved layouts.
func (p *FiniteIntervalProduct) ToFormula() (result ast.ASTRef, resultErr error) {
	if err := p.open(); err != nil {
		return result, err
	}
	m := p.manager.manager
	rows, err := p.table.Facts()
	if err != nil {
		return result, err
	}
	var roots []ast.ASTRef
	owned := false
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&roots[i]))
		}
		if resultErr != nil && owned {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	hold := func(value ast.ASTHandle, cause error) (ast.ASTHandle, error) {
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		ref, cause := m.NewRef(value)
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		roots = append(roots, ref)
		return value, nil
	}
	shifter, err := rewriter.NewVarShifter(m)
	if shifter != nil {
		defer func() { resultErr = errors.Join(resultErr, shifter.Close()) }()
	}
	if err != nil {
		return result, err
	}
	innerSorts := make([]ast.ASTHandle, len(p.intervalColumns))
	for i, column := range p.intervalColumns {
		innerSorts[i] = p.sorts[column]
	}
	disjunction := make([]ast.ASTHandle, 0, len(rows))
	for _, row := range rows {
		index := uint64(row[len(row)-1])
		if index >= uint64(len(p.intervals)) || p.intervals[index] == nil {
			return result, ErrTableRelation
		}
		inner, err := p.intervals[index].ToFormula(m, innerSorts)
		if err != nil {
			return result, err
		}
		roots = append(roots, inner)
		innerHandle, err := m.RefGet(inner)
		if err != nil {
			return result, err
		}
		shifted, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return result, err
		}
		roots = append(roots, shifted)
		if err = shifter.ShiftBy(innerHandle, uint32(len(p.tableColumns)), shifted); err != nil {
			return result, err
		}
		conjuncts := make([]ast.ASTHandle, 0, len(row))
		for i, value := range row[:len(row)-1] {
			variable, err := hold(m.MkVar(uint32(i), p.sorts[i]))
			if err != nil {
				return result, err
			}
			numeral, err := hold(m.MkDatalogNumeral(uint64(value), p.sorts[i]))
			if err != nil {
				return result, err
			}
			equality, err := hold(m.MkEq(variable, numeral))
			if err != nil {
				return result, err
			}
			conjuncts = append(conjuncts, equality)
		}
		shiftedHandle, err := m.RefGet(shifted)
		if err != nil {
			return result, err
		}
		conjuncts = append(conjuncts, shiftedHandle)
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
	owned = true
	resultErr = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpOr, disjunction, result)
	return result, resultErr
}
