package qe

// Source: src/qe/qe_arith_plugin.cpp arith_qe_util constructors.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (u *ArithmeticQETermUtil) scale(value *big.Rat, term ast.ASTHandle) (ast.ASTRef, error) {
	if value == nil {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	if value.Cmp(big.NewRat(1, 1)) == 0 {
		return u.manager.NewRef(term)
	}
	numeral, err := u.numeralFor(value, term)
	if err != nil {
		return ast.ASTRef{}, err
	}
	product, err := u.manager.MkArithmeticMul(numeral, term)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return u.manager.NewRef(product)
}

func (u *ArithmeticQETermUtil) comparisonZero(expression ast.ASTHandle, strict bool) (ast.ASTRef, error) {
	zero, err := u.zero(expression)
	if err != nil {
		return ast.ASTRef{}, err
	}
	var comparison ast.ASTHandle
	if strict {
		comparison, err = u.manager.MkArithmeticLT(expression, zero)
	} else {
		comparison, err = u.manager.MkArithmeticLE(expression, zero)
	}
	if err != nil {
		return ast.ASTRef{}, err
	}
	return u.manager.NewRef(comparison)
}

// MakeBound constructs b*t <= a*s. Strict integer bounds use e+1 <= 0.
func (u *ArithmeticQETermUtil) MakeBound(a *big.Rat, t ast.ASTHandle, b *big.Rat, s ast.ASTHandle, strict bool) (result ast.ASTRef, resultErr error) {
	if resultErr = u.open(); resultErr != nil {
		return result, resultErr
	}
	if a == nil || b == nil || a.Sign() != b.Sign() {
		return result, ast.ErrASTDomain
	}
	absA, absB := new(big.Rat).Abs(a), new(big.Rat).Abs(b)
	ss, err := u.scale(absA, s)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, u.manager.DelRef(&ss)) }()
	tt, err := u.scale(absB, t)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, u.manager.DelRef(&tt)) }()
	sh, err := u.manager.RefGet(ss)
	if err != nil {
		return result, err
	}
	th, err := u.manager.RefGet(tt)
	if err != nil {
		return result, err
	}
	left, right := sh, th
	if a.Sign() < 0 {
		left, right = th, sh
	}
	difference, err := u.manager.MkArithmeticSub(left, right)
	if err != nil {
		return result, err
	}
	differenceRef, err := u.manager.NewRef(difference)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, u.manager.DelRef(&differenceRef)) }()
	integer, err := u.isInteger(difference)
	if err != nil {
		return result, err
	}
	if strict && integer {
		one, cause := u.manager.ArithmeticIntegerNumeralText("1")
		if cause != nil {
			return result, cause
		}
		shifted, cause := u.manager.MkArithmeticAdd(difference, one)
		if cause != nil {
			return result, cause
		}
		shiftedRef, cause := u.manager.NewRef(shifted)
		if cause != nil {
			return result, cause
		}
		comparison, cause := u.comparisonZeroOwned(shifted, false)
		return comparison, errors.Join(cause, u.manager.DelRef(&shiftedRef))
	}
	return u.comparisonZeroOwned(difference, strict)
}

func (u *ArithmeticQETermUtil) comparisonZeroOwned(expression ast.ASTHandle, strict bool) (ast.ASTRef, error) {
	return u.comparisonZero(expression, strict)
}

// MakeAbs retains the source numeral shortcut and ite fallback.
func (u *ArithmeticQETermUtil) MakeAbs(expression ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if resultErr = u.open(); resultErr != nil {
		return result, resultErr
	}
	if value, numeral, err := u.numeral(expression); err != nil {
		return result, err
	} else if numeral {
		if value.Sign() >= 0 {
			return u.manager.NewRef(expression)
		}
		negative, err := u.manager.MkArithmeticNeg(expression)
		if err != nil {
			return result, err
		}
		return u.manager.NewRef(negative)
	}
	zero, err := u.zero(expression)
	if err != nil {
		return result, err
	}
	nonnegative, err := u.manager.MkArithmeticLE(zero, expression)
	if err != nil {
		return result, err
	}
	nonnegativeRef, err := u.manager.NewRef(nonnegative)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, u.manager.DelRef(&nonnegativeRef)) }()
	negative, err := u.manager.MkArithmeticNeg(expression)
	if err != nil {
		return result, err
	}
	abs, err := u.manager.MkIte(nonnegative, expression, negative)
	if err != nil {
		return result, err
	}
	return u.manager.NewRef(abs)
}

func (u *ArithmeticQETermUtil) minMax(arguments []ast.ASTHandle, maximum bool) (result ast.ASTRef, resultErr error) {
	if resultErr = u.open(); resultErr != nil {
		return result, resultErr
	}
	if len(arguments) == 0 {
		return result, ast.ErrASTDomain
	}
	result, resultErr = u.manager.NewRef(arguments[len(arguments)-1])
	if resultErr != nil {
		return result, resultErr
	}
	for i := len(arguments) - 2; i >= 0; i-- {
		current, err := u.manager.RefGet(result)
		if err != nil {
			return result, errors.Join(err, u.manager.DelRef(&result))
		}
		left, right := arguments[i], current
		if !maximum {
			left, right = current, arguments[i]
		}
		comparison, err := u.manager.MkArithmeticLE(left, right)
		if err != nil {
			return result, errors.Join(err, u.manager.DelRef(&result))
		}
		choice, err := u.manager.MkIte(comparison, current, arguments[i])
		if err != nil {
			return result, errors.Join(err, u.manager.DelRef(&result))
		}
		if err = u.manager.SetRef(result, choice); err != nil {
			return result, errors.Join(err, u.manager.DelRef(&result))
		}
	}
	return result, nil
}

func (u *ArithmeticQETermUtil) MakeMax(arguments []ast.ASTHandle) (ast.ASTRef, error) {
	return u.minMax(arguments, true)
}

func (u *ArithmeticQETermUtil) MakeMin(arguments []ast.ASTHandle) (ast.ASTRef, error) {
	return u.minMax(arguments, false)
}

func (u *ArithmeticQETermUtil) MakeDivides(divisor *big.Rat, expression ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if resultErr = u.open(); resultErr != nil {
		return result, resultErr
	}
	if divisor == nil || !divisor.IsInt() {
		return result, ast.ErrASTDomain
	}
	k, err := u.numeralFor(divisor, expression)
	if err != nil {
		return result, err
	}
	modular, err := u.manager.MkFamilyApp(u.family, 13, nil, []ast.ASTHandle{expression, k})
	if err != nil {
		return result, err
	}
	modularRef, err := u.manager.NewRef(modular)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, u.manager.DelRef(&modularRef)) }()
	zero, err := u.zero(expression)
	if err != nil {
		return result, err
	}
	equality, err := u.manager.MkEq(zero, modular)
	if err != nil {
		return result, err
	}
	return u.manager.NewRef(equality)
}

func (u *ArithmeticQETermUtil) MakeFlatAnd(left, right ast.ASTHandle) (ast.ASTRef, error) {
	if err := u.open(); err != nil {
		return ast.ASTRef{}, err
	}
	parts := make([]ast.ASTHandle, 0, 4)
	for _, expression := range []ast.ASTHandle{left, right} {
		args, conjunction, err := u.operation(expression, u.manager.BasicFamilyID(), ast.ASTOpAnd)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if conjunction {
			parts = append(parts, args...)
		} else {
			parts = append(parts, expression)
		}
	}
	combined, err := u.manager.MkAndArray(parts)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return u.manager.NewRef(combined)
}
