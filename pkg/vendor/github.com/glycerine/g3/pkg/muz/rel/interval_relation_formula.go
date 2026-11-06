package rel

// Source: src/muz/rel/dl_interval_relation.cpp interval_relation::to_formula.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// ToFormula builds the source conjunction over column variables and bounds.
// The returned AST reference is owned by the caller.
func (r *IntervalRelation) ToFormula(m *ast.ASTManager, sorts []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if r == nil || r.values == nil || m == nil || len(sorts) != len(r.values.columns) {
		return result, ast.ErrASTDomain
	}
	family, err := m.RegisterArithmeticPlugin()
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
	hold := func(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := m.NewRef(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		roots = append(roots, ref)
		return h, nil
	}
	variables := make([]ast.ASTHandle, len(sorts))
	integers := make([]bool, len(sorts))
	for i, sort := range sorts {
		fid, e := m.GetFamilyID(sort)
		if e != nil {
			return result, e
		}
		kind, e := m.GetDeclKind(sort)
		if e != nil {
			return result, e
		}
		if fid != family || kind < 0 || kind > 1 {
			return result, ast.ErrASTDomain
		}
		integers[i] = kind == 1
		variables[i], e = hold(m.MkVar(uint32(i), sort))
		if e != nil {
			return result, e
		}
	}
	var conjuncts []ast.ASTHandle
	add := func(h ast.ASTHandle, e error) error {
		value, e := hold(h, e)
		if e == nil {
			conjuncts = append(conjuncts, value)
		}
		return e
	}
	for i := range variables {
		root := r.values.find(uint32(i))
		if root != uint32(i) {
			if err = add(m.MkEq(variables[i], variables[root])); err != nil {
				return result, err
			}
			continue
		}
		lower, lowerOpen, upper, upperOpen := r.values.columns[i].Bounds()
		if lower != nil {
			bound, e := hold(m.ArithmeticNumeralRat(lower, integers[i]))
			if e != nil {
				return result, e
			}
			if lowerOpen {
				err = add(m.MkArithmeticLT(bound, variables[i]))
			} else {
				err = add(m.MkArithmeticLE(bound, variables[i]))
			}
			if err != nil {
				return result, err
			}
		}
		if upper != nil {
			bound, e := hold(m.ArithmeticNumeralRat(upper, integers[i]))
			if e != nil {
				return result, e
			}
			if upperOpen {
				err = add(m.MkArithmeticLT(variables[i], bound))
			} else {
				err = add(m.MkArithmeticLE(variables[i], bound))
			}
			if err != nil {
				return result, err
			}
		}
	}
	result, resultErr = m.NewRef(ast.ASTHandle{})
	if resultErr != nil {
		return result, resultErr
	}
	owned = true
	resultErr = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpAnd, conjuncts, result)
	return result, resultErr
}
