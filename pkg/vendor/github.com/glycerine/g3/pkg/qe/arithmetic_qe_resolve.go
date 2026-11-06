package qe

// Source: src/qe/qe_arith_plugin.cpp arith_qe_util::mk_resolve,
// dependency-closed unit/real branch before Cooper slack expansion.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// ResolveUnitOrReal eliminates opposite-signed occurrences from two bounds.
// Integer bounds with two non-unit coefficients use the bounded-variable Cooper expansion when a QE context is available.
func (u *ArithmeticQETermUtil) ResolveUnitOrReal(variable ast.ASTHandle, strict bool, a *big.Rat, t ast.ASTHandle, b *big.Rat, s ast.ASTHandle) (result ast.ASTRef, handled bool, resultErr error) {
	if resultErr = u.open(); resultErr != nil {
		return result, false, resultErr
	}
	if a == nil || b == nil || a.Sign() == 0 || b.Sign() == 0 || a.Sign() == b.Sign() {
		return result, false, ast.ErrASTDomain
	}
	absA, absB := new(big.Rat).Abs(a), new(big.Rat).Abs(b)
	integer, err := u.isInteger(variable)
	if err != nil {
		return result, false, err
	}
	one := big.NewRat(1, 1)
	if integer && absA.Cmp(one) != 0 && absB.Cmp(one) != 0 {
		if strict {
			return result, false, ast.ErrASTDomain
		}
		return u.resolveCooper(variable, a, t, b, s)
	}
	if strict && (absA.Cmp(one) != 0 || absB.Cmp(one) != 0) {
		return result, false, ast.ErrASTDomain
	}
	bt, err := u.scale(absB, t)
	if err != nil {
		return result, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, u.manager.DelRef(&bt)) }()
	as, err := u.scale(absA, s)
	if err != nil {
		return result, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, u.manager.DelRef(&as)) }()
	bth, err := u.manager.RefGet(bt)
	if err != nil {
		return result, false, err
	}
	ash, err := u.manager.RefGet(as)
	if err != nil {
		return result, false, err
	}
	sum, err := u.manager.MkArithmeticAdd(ash, bth)
	if err != nil {
		return result, false, err
	}
	sumRef, err := u.manager.NewRef(sum)
	if err != nil {
		return result, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, u.manager.DelRef(&sumRef)) }()
	result, err = u.comparisonZero(sum, strict)
	return result, err == nil, err
}
