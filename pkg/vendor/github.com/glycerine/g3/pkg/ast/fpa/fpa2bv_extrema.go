package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp:1181-1296,4072-4103.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *Converter) minMaxSignPair(declaration ast.ASTHandle) ([2]ast.ASTHandle, error) {
	if pair, found := c.minMaxSigns[declaration]; found {
		return pair, nil
	}
	sort, err := c.manager.BitVectorSort(1)
	if err != nil {
		return [2]ast.ASTHandle{}, err
	}
	first, err := c.manager.MkFreshConstNullable(nil, sort)
	if err != nil {
		return [2]ast.ASTHandle{}, err
	}
	second, err := c.manager.MkFreshConstNullable(nil, sort)
	if err != nil {
		return [2]ast.ASTHandle{}, err
	}
	if err = c.manager.IncRef(declaration); err != nil {
		return [2]ast.ASTHandle{}, err
	}
	if err = c.manager.IncRef(first); err != nil {
		_ = c.manager.DecRef(declaration)
		return [2]ast.ASTHandle{}, err
	}
	if err = c.manager.IncRef(second); err != nil {
		return [2]ast.ASTHandle{}, errors.Join(err, c.manager.DecRef(first), c.manager.DecRef(declaration))
	}
	pair := [2]ast.ASTHandle{first, second}
	c.minMaxSigns[declaration] = pair
	return pair, nil
}

func (c *Converter) minMaxUnspecified(declaration, left, right ast.ASTHandle) (ast.ASTHandle, error) {
	rangeSort, err := c.manager.GetRange(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ebits, sbits, matched, err := c.manager.FloatingPointSortShape(rangeSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !matched {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	pair, err := c.minMaxSignPair(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zeroExponent, err := c.zero(ebits)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zeroSignificand, err := c.zero(sbits - 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	positiveNegative, err := c.MkFP(pair[0], zeroExponent, zeroSignificand)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negativePositive, err := c.MkFP(pair[1], zeroExponent, zeroSignificand)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	leftPositive, err := c.isPositiveSign(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	leftZero, err := c.isZero(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	leftPositiveZero, err := c.boolAnd(leftPositive, leftZero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightNegative, err := c.isNegativeSign(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightZero, err := c.isZero(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightNegativeZero, err := c.boolAnd(rightNegative, rightZero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	positiveThenNegative, err := c.boolAnd(leftPositiveZero, rightNegativeZero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.boolITE(positiveThenNegative, positiveNegative, negativePositive)
}

// Extremum converts fp.min or fp.max, including the source's stable fresh
// result for the opposite-signed-zero unspecified case.
func (c *Converter) Extremum(declaration ast.ASTHandle, kind int32, left, right ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if kind != ast.FPAOpMin && kind != ast.FPAOpMax {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	leftNaN, err := c.isNaN(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightNaN, err := c.isNaN(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	leftZero, err := c.isZero(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightZero, err := c.isZero(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bothZero, err := c.boolAnd(leftZero, rightZero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	leftSign, _, _, err := c.SplitFP(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightSign, _, _, err := c.SplitFP(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	signsEqual, err := c.boolEq(leftSign, rightSign)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	signsDiffer, err := c.boolNot(signsEqual)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	oppositeZeros, err := c.boolAnd(bothZero, signsDiffer)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	unspecified, err := c.minMaxUnspecified(declaration, left, right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	comparisonKind := ast.FPAOpLt
	if kind == ast.FPAOpMax {
		comparisonKind = ast.FPAOpGt
	}
	ordered, err := c.Compare(comparisonKind, left, right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := c.boolITE(ordered, left, right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err = c.boolITE(bothZero, right, result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err = c.boolITE(oppositeZeros, unspecified, result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err = c.boolITE(rightNaN, left, result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.boolITE(leftNaN, right, result)
}
