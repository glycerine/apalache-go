package opt

// Source: src/opt/opt_context.cpp:1352-1381, context::to_expr, adapted to
// the Go simplex value used by the retained arithmetic theories.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
)

// ArithmeticValueExpression converts c*infinity+a+b*epsilon into an owned AST
// reference suitable for SMT-LIB objective display.
func ArithmeticValueExpression(m *ast.ASTManager, value *simplex.Infinitesimal, integer bool) (_ ast.ASTRef, result error) {
	if m == nil || value == nil {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	var args []ast.ASTHandle
	var roots []ast.ASTRef
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			result = errors.Join(result, m.DelRef(&roots[i]))
		}
	}()
	hold := func(h ast.ASTHandle, err error) error {
		if err != nil {
			return err
		}
		ref, err := m.NewRef(h)
		if err != nil {
			return err
		}
		roots = append(roots, ref)
		args = append(args, h)
		return nil
	}
	coefficientTerm := func(coefficient *big.Rat, name string, termInteger bool) (ast.ASTHandle, error) {
		family, err := m.ArithmeticFamilyID()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		sortKind := int32(0)
		if termInteger {
			sortKind = 1
		}
		sort, err := m.MkFamilySort(family, sortKind, nil)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		symbol, err := m.Symbols.Intern(name)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		term, err := m.MkConst(symbol, sort)
		if err != nil || coefficient.Cmp(big.NewRat(1, 1)) == 0 {
			return term, err
		}
		numeral, err := m.ArithmeticNumeralRat(coefficient, termInteger)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return m.MkArithmeticMul(numeral, term)
	}
	if value.Infinity.Sign() != 0 {
		term, err := coefficientTerm(&value.Infinity, "oo", integer && value.Epsilon.Sign() == 0)
		if err = hold(term, err); err != nil {
			return ast.ASTRef{}, err
		}
	}
	if value.Main.Sign() != 0 {
		if err := hold(m.ArithmeticNumeralRat(&value.Main, integer && value.Epsilon.Sign() == 0)); err != nil {
			return ast.ASTRef{}, err
		}
	}
	if value.Epsilon.Sign() != 0 {
		term, err := coefficientTerm(&value.Epsilon, "epsilon", false)
		if err = hold(term, err); err != nil {
			return ast.ASTRef{}, err
		}
	}
	var expression ast.ASTHandle
	var err error
	switch len(args) {
	case 0:
		expression, err = m.ArithmeticNumeralRat(new(big.Rat), integer)
	case 1:
		expression = args[0]
	default:
		expression, err = m.MkArithmeticAdd(args[0], args[1])
		for i := 2; err == nil && i < len(args); i++ {
			expression, err = m.MkArithmeticAdd(expression, args[i])
		}
	}
	if err != nil {
		return ast.ASTRef{}, err
	}
	return m.NewRef(expression)
}
