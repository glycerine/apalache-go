package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp:652-666,1168-1179,2065-2261,3349-3512.

import "github.com/glycerine/g3/pkg/ast"

func (c *Converter) zero(width uint32) (ast.ASTHandle, error) {
	return c.manager.BitVectorNumeralUnsigned(0, width)
}

func (c *Converter) one(width uint32) (ast.ASTHandle, error) {
	return c.manager.BitVectorNumeralUnsigned(1, width)
}

func (c *Converter) isInfinite(value ast.ASTHandle) (ast.ASTHandle, error) {
	_, exponent, significand, err := c.SplitFP(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ew, err := c.manager.BitVectorSize(exponent)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sw, err := c.manager.BitVectorSize(significand)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	top, err := c.manager.BitVectorNumeralAllOnes(ew)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zero, err := c.zero(sw)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sigZero, err := c.boolEq(significand, zero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	expTop, err := c.boolEq(exponent, top)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.boolAnd(sigZero, expTop)
}

func (c *Converter) isPositiveSign(value ast.ASTHandle) (ast.ASTHandle, error) {
	sign, _, _, err := c.SplitFP(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zero, err := c.zero(1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.boolEq(sign, zero)
}

func (c *Converter) isNegativeSign(value ast.ASTHandle) (ast.ASTHandle, error) {
	sign, _, _, err := c.SplitFP(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	one, err := c.one(1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.boolEq(sign, one)
}

func (c *Converter) isZero(value ast.ASTHandle) (ast.ASTHandle, error) {
	_, exponent, significand, err := c.SplitFP(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ew, err := c.manager.BitVectorSize(exponent)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sw, err := c.manager.BitVectorSize(significand)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zeroE, err := c.zero(ew)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zeroS, err := c.zero(sw)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	expZero, err := c.boolEq(exponent, zeroE)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sigZero, err := c.boolEq(significand, zeroS)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.boolAnd(sigZero, expZero)
}

func (c *Converter) isSubnormal(value ast.ASTHandle) (ast.ASTHandle, error) {
	_, exponent, _, err := c.SplitFP(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ew, err := c.manager.BitVectorSize(exponent)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zero, err := c.zero(ew)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	expZero, err := c.boolEq(exponent, zero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	valueZero, err := c.isZero(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	notZero, err := c.boolNot(valueZero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.boolAnd(notZero, expZero)
}

func (c *Converter) isNormal(value ast.ASTHandle) (ast.ASTHandle, error) {
	_, exponent, _, err := c.SplitFP(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ew, err := c.manager.BitVectorSize(exponent)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	top, err := c.manager.BitVectorNumeralAllOnes(ew)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	special, err := c.boolEq(exponent, top)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	subnormal, err := c.isSubnormal(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zero, err := c.isZero(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	excluded, err := c.boolOr(special, subnormal, zero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.boolNot(excluded)
}

// Classify produces the bit-vector formula for an FPA classification predicate.
func (c *Converter) Classify(value ast.ASTHandle, kind ast.FPAClassificationKind) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	switch kind {
	case ast.FPAIsNaN:
		return c.isNaN(value)
	case ast.FPAIsInfinite:
		return c.isInfinite(value)
	case ast.FPAIsZero:
		return c.isZero(value)
	case ast.FPAIsNormal:
		return c.isNormal(value)
	case ast.FPAIsSubnormal:
		return c.isSubnormal(value)
	case ast.FPAIsNegative, ast.FPAIsPositive:
		nan, err := c.isNaN(value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		notNaN, err := c.boolNot(nan)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		var sign ast.ASTHandle
		if kind == ast.FPAIsNegative {
			sign, err = c.isNegativeSign(value)
		} else {
			sign, err = c.isPositiveSign(value)
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return c.boolAnd(notNaN, sign)
	default:
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
}

// Negate flips the sign of every encoded value except NaN, matching the source
// converter's canonical-NaN preservation rule.
func (c *Converter) Negate(value ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	sign, exponent, significand, err := c.SplitFP(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	nan, err := c.isNaN(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negated, err := c.manager.MkBitVectorNot(sign)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	resultSign, err := c.boolITE(nan, sign, negated)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.MkFP(resultSign, exponent, significand)
}

// Abs clears the encoded sign bit, including for NaN.
func (c *Converter) Abs(value ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	_, exponent, significand, err := c.SplitFP(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zero, err := c.zero(1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.MkFP(zero, exponent, significand)
}

func (c *Converter) unsignedLT(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	notLess, err := c.manager.MkBitVectorULE(right, left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.boolNot(notLess)
}

func (c *Converter) floatEqual(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	leftNaN, err := c.isNaN(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightNaN, err := c.isNaN(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	nan, err := c.boolOr(leftNaN, rightNaN)
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
	ls, le, lm, err := c.SplitFP(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rs, re, rm, err := c.SplitFP(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	eqs, err := c.boolEq(ls, rs)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	eqe, err := c.boolEq(le, re)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	eqm, err := c.boolEq(lm, rm)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	same, err := c.boolAnd(eqs, eqe, eqm)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	truth, err := c.manager.MkTrue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	falsity, err := c.manager.MkFalse()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	nonNaN, err := c.boolITE(bothZero, truth, same)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.boolITE(nan, falsity, nonNaN)
}

func (c *Converter) floatLess(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	leftNaN, err := c.isNaN(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightNaN, err := c.isNaN(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	nan, err := c.boolOr(leftNaN, rightNaN)
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
	ls, le, lm, err := c.SplitFP(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rs, re, rm, err := c.SplitFP(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zero1, err := c.zero(1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	one1, err := c.one(1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	leftNegative, err := c.boolEq(ls, one1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightPositive, err := c.boolEq(rs, zero1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightNegative, err := c.boolEq(rs, one1)
	if err != nil {
		return ast.ASTHandle{}, err
	}

	rExpLess, err := c.unsignedLT(re, le)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rSigLess, err := c.unsignedLT(rm, lm)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	expEqual, err := c.boolEq(re, le)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rTailLess, err := c.boolAnd(expEqual, rSigLess)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	reversedMagnitude, err := c.boolOr(rExpLess, rTailLess)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	truth, err := c.manager.MkTrue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negativeCase, err := c.boolITE(rightPositive, truth, reversedMagnitude)
	if err != nil {
		return ast.ASTHandle{}, err
	}

	lExpLess, err := c.unsignedLT(le, re)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	lSigLess, err := c.unsignedLT(lm, rm)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	expEqual, err = c.boolEq(le, re)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	lTailLess, err := c.boolAnd(expEqual, lSigLess)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	forwardMagnitude, err := c.boolOr(lExpLess, lTailLess)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	falsity, err := c.manager.MkFalse()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	positiveCase, err := c.boolITE(rightNegative, falsity, forwardMagnitude)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ordered, err := c.boolITE(leftNegative, negativeCase, positiveCase)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	nonzero, err := c.boolITE(bothZero, falsity, ordered)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.boolITE(nan, falsity, nonzero)
}

// Compare converts fp.eq, fp.lt, fp.gt, fp.leq and fp.geq to Boolean/BV form.
func (c *Converter) Compare(kind int32, left, right ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	switch kind {
	case ast.FPAOpEq:
		return c.floatEqual(left, right)
	case ast.FPAOpLt:
		return c.floatLess(left, right)
	case ast.FPAOpLe:
		less, err := c.floatLess(left, right)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		equal, err := c.floatEqual(left, right)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return c.boolOr(less, equal)
	case ast.FPAOpGt:
		return c.floatLess(right, left)
	case ast.FPAOpGe:
		greater, err := c.floatLess(right, left)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		equal, err := c.floatEqual(left, right)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return c.boolOr(greater, equal)
	default:
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
}
