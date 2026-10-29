package smtcore

// Source: src/smt/theory_arith_core.h is_int_expr/mk_var and
// src/smt/theory_arith_int.h integer bound rounding and branch selection.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func floorSMTRational(value *big.Rat) *big.Int {
	floor := new(big.Int).Quo(value.Num(), value.Denom())
	if value.Sign() < 0 && !value.IsInt() {
		floor.Sub(floor, big.NewInt(1))
	}
	return floor
}

// Preserve the source bounded traversal, including integral Real numerals,
// to_real terms, and sums/products of integral expressions.
func (t *SMTLinearArithmeticTheory) isIntegerExpression(expression ast.ASTHandle) (bool, error) {
	m := t.state.manager
	sort, err := m.GetSort(expression)
	if err != nil {
		return false, err
	}
	kind, err := m.GetDeclKind(sort)
	if err != nil {
		return false, err
	}
	if kind == 1 {
		return true, nil
	}
	stack := []ast.ASTHandle{expression}
	visited := 0
	for len(stack) != 0 {
		visited++
		if visited > 100 {
			return false, nil
		}
		i := len(stack) - 1
		e := stack[i]
		stack = stack[:i]
		op, own, err := t.operation(e)
		if err != nil {
			return false, err
		}
		if !own {
			return false, nil
		}
		if op == smtArithOpToReal {
			continue
		}
		value, numeral, err := t.numeral(e)
		if err != nil {
			return false, err
		}
		if numeral && value.IsInt() {
			continue
		}
		if op != smtArithOpAdd && op != smtArithOpMul {
			return false, nil
		}
		n, err := m.GetNumArgs(e)
		if err != nil {
			return false, err
		}
		for j := uint32(0); j < n; j++ {
			arg, err := m.GetArg(e, j)
			if err != nil {
				return false, err
			}
			stack = append(stack, arg)
		}
	}
	return true, nil
}

// The constant is moved to the bound. Integral coefficients on integral
// columns make the remaining polynomial integer-valued even in a Real atom.
func (t *SMTLinearArithmeticTheory) integralAtom(atom *smtLinearArithmeticAtom) bool {
	if atom.native != nil {
		return t.integer || t.integerColumns[t.simplexVariables[atom.native.variable]]
	}
	if t.integer {
		return true
	}
	if !t.mixed {
		return false
	}
	for column, coefficient := range atom.form.terms {
		if !t.integerColumns[column] || !coefficient.IsInt() {
			return false
		}
	}
	return true
}
