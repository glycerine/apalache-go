package qe

// Source: src/qe/qe_arith_plugin.cpp arith_qe_util relation matching.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (u *ArithmeticQETermUtil) Negated(expression ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if err := u.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	args, matched, err := u.operation(expression, u.manager.BasicFamilyID(), ast.ASTOpNot)
	if err != nil || !matched || len(args) != 1 {
		return ast.ASTHandle{}, false, err
	}
	return args[0], true, nil
}

func (u *ArithmeticQETermUtil) comparisonPolynomial(expression ast.ASTHandle, wantLE bool) (result ast.ASTRef, matched bool, err error) {
	if err = u.open(); err != nil {
		return result, false, err
	}
	family, err := u.manager.GetFamilyID(expression)
	if err != nil || family != u.family {
		return result, false, err
	}
	kind, err := u.manager.GetDeclKind(expression)
	if err != nil || kind != 2 && kind != 3 {
		return result, false, err
	}
	left, err := u.manager.GetArg(expression, 0)
	if err != nil {
		return result, false, err
	}
	right, err := u.manager.GetArg(expression, 1)
	if err != nil {
		return result, false, err
	}
	polynomial, other := left, right
	if kind == 2 && !wantLE || kind == 3 && wantLE {
		polynomial, other = right, left
	}
	if value, numeral, cause := u.numeral(other); cause != nil {
		return result, false, cause
	} else if numeral && value.Sign() == 0 {
		result, cause = u.manager.NewRef(polynomial)
		if cause == nil && u.theory != nil {
			cause = u.theory.Rewrite(polynomial, result)
			if cause != nil {
				cause = errors.Join(cause, u.manager.DelRef(&result))
			}
		}
		return result, cause == nil, cause
	}
	difference, err := u.manager.MkArithmeticSub(polynomial, other)
	if err != nil {
		return result, false, err
	}
	result, err = u.manager.NewRef(difference)
	if err == nil && u.theory != nil {
		err = u.theory.Rewrite(difference, result)
		if err != nil {
			_ = u.manager.DelRef(&result)
			return ast.ASTRef{}, false, err
		}
	}
	return result, err == nil, err
}

// MatchLE normalizes <= and reversed >= to p <= 0.
func (u *ArithmeticQETermUtil) MatchLE(expression ast.ASTHandle) (ast.ASTRef, bool, error) {
	return u.comparisonPolynomial(expression, true)
}

// MatchGE normalizes >= and reversed <= to p >= 0.
func (u *ArithmeticQETermUtil) MatchGE(expression ast.ASTHandle) (ast.ASTRef, bool, error) {
	return u.comparisonPolynomial(expression, false)
}

// MatchLT normalizes < and reversed > to p < 0.
func (u *ArithmeticQETermUtil) MatchLT(expression ast.ASTHandle) (result ast.ASTRef, matched bool, err error) {
	if err = u.open(); err != nil {
		return result, false, err
	}
	family, err := u.manager.GetFamilyID(expression)
	if err != nil || family != u.family {
		return result, false, err
	}
	kind, err := u.manager.GetDeclKind(expression)
	if err != nil || kind != 4 && kind != 5 {
		return result, false, err
	}
	left, err := u.manager.GetArg(expression, 0)
	if err != nil {
		return result, false, err
	}
	right, err := u.manager.GetArg(expression, 1)
	if err != nil {
		return result, false, err
	}
	if kind == 5 {
		left, right = right, left
	}
	if value, numeral, cause := u.numeral(right); cause != nil {
		return result, false, cause
	} else if numeral && value.Sign() == 0 {
		result, cause = u.manager.NewRef(left)
		if cause == nil && u.theory != nil {
			cause = u.theory.Rewrite(left, result)
			if cause != nil {
				cause = errors.Join(cause, u.manager.DelRef(&result))
			}
		}
		return result, cause == nil, cause
	}
	difference, err := u.manager.MkArithmeticSub(left, right)
	if err != nil {
		return result, false, err
	}
	result, err = u.manager.NewRef(difference)
	if err == nil && u.theory != nil {
		err = u.theory.Rewrite(difference, result)
		if err != nil {
			_ = u.manager.DelRef(&result)
			return ast.ASTRef{}, false, err
		}
	}
	return result, err == nil, err
}

func (u *ArithmeticQETermUtil) dividesSide(zero, modular ast.ASTHandle) (divisor *big.Rat, term ast.ASTHandle, matched bool, err error) {
	value, numeral, err := u.numeral(zero)
	if err != nil || !numeral || value.Sign() != 0 {
		return nil, term, false, err
	}
	args, mod, err := u.operation(modular, u.family, 13)
	if err != nil || !mod || len(args) != 2 {
		return nil, term, false, err
	}
	divisor, numeral, err = u.numeral(args[1])
	if err != nil || !numeral {
		return nil, term, false, err
	}
	return divisor, args[0], true, nil
}

// MatchDivides recognizes 0 = p mod k in either equality orientation.
func (u *ArithmeticQETermUtil) MatchDivides(expression ast.ASTHandle) (*big.Rat, ast.ASTHandle, bool, error) {
	if err := u.open(); err != nil {
		return nil, ast.ASTHandle{}, false, err
	}
	args, equality, err := u.operation(expression, u.manager.BasicFamilyID(), ast.ASTOpEq)
	if err != nil || !equality || len(args) != 2 {
		return nil, ast.ASTHandle{}, false, err
	}
	if divisor, term, matched, cause := u.dividesSide(args[0], args[1]); cause != nil || matched {
		return divisor, term, matched, cause
	}
	return u.dividesSide(args[1], args[0])
}

func (u *ArithmeticQETermUtil) MatchNotDivides(expression ast.ASTHandle) (atom ast.ASTHandle, divisor *big.Rat, term ast.ASTHandle, matched bool, err error) {
	inner, negative, err := u.Negated(expression)
	if err != nil || !negative {
		return atom, nil, term, false, err
	}
	divisor, term, matched, err = u.MatchDivides(inner)
	return inner, divisor, term, matched, err
}
