package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp:143-150,244-276;
// src/util/rational.h:400-412,503-505.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// rationalPowerTwo uses source power(rational(2), exponent), whose general
// exponentiation includes the final unused square. It does not use the
// separate rational::power_of_two cache.
func (c *bv2RealCall) rationalPowerTwo(exponent uint32) *unumeral.OwnedRational {
	if c.err != nil {
		return nil
	}
	count := len(c.numbers)
	s := c.u.manager.NumeralService()
	two := c.number()
	if c.err == nil {
		c.err = s.SetInt64(two, 2)
	}
	if c.err != nil {
		c.err = errors.Join(c.err, c.releaseNumbersTo(count))
		return nil
	}
	value, err := s.Power(two, exponent)
	c.err = errors.Join(err, c.releaseNumbersTo(count))
	return c.takeRational(value, nil)
}

func (c *bv2RealCall) signedNumeral(q *big.Rat) (result ast.ASTHandle) {
	mark := c.checkpoint()
	defer c.endExpressionScope(mark, &result)
	if c.err != nil {
		return ast.ASTHandle{}
	}
	value := c.rational(q)
	if c.err != nil {
		return ast.ASTHandle{}
	}
	return c.signedNumeralOwned(value)
}

func (c *bv2RealCall) signedNumeralOwned(value *unumeral.OwnedRational) (result ast.ASTHandle) {
	mark := c.checkpoint()
	defer c.endExpressionScope(mark, &result)
	if c.err != nil {
		return ast.ASTHandle{}
	}
	integer, err := c.u.manager.NumeralService().IsInt(value)
	if err != nil {
		c.err = err
		return ast.ASTHandle{}
	}
	if c.err = bv2RealSourceAssert(245, "n.is_int()", integer); c.err != nil {
		return ast.ASTHandle{}
	}
	if !integer {
		c.err = ast.ErrASTDomain
		return ast.ASTHandle{}
	}
	s := c.u.manager.NumeralService()
	negative, err := s.IsNeg(value)
	if err != nil {
		c.err = err
		return ast.ASTHandle{}
	}
	if negative {
		value = c.takeRational(s.Abs(value))
		if c.err != nil {
			return ast.ASTHandle{}
		}
	}
	width, err := s.NumBits(value)
	if err != nil {
		c.err = err
		return ast.ASTHandle{}
	}
	h := c.hold(c.u.manager.BitVectorNumeral(value, width+1))
	if negative {
		h = c.bv(3, h)
	}
	return h
}

func (c *bv2RealCall) extend(n uint32, h ast.ASTHandle) (result ast.ASTHandle) {
	mark := c.checkpoint()
	defer c.endExpressionScope(mark, &result)
	if c.err != nil {
		return ast.ASTHandle{}
	}
	if n == 0 {
		return h
	}
	value := c.number()
	if c.err != nil {
		return ast.ASTHandle{}
	}
	var width uint32
	numeral, err := c.u.manager.BitVectorNumeralValue(h, value, &width)
	if err != nil {
		c.err = err
		return ast.ASTHandle{}
	}
	if numeral {
		limit := c.rationalPowerTwo(width - 1)
		if c.err != nil {
			return ast.ASTHandle{}
		}
		positive, err := c.u.manager.NumeralService().Greater(limit, value)
		if err != nil {
			c.err = err
			return ast.ASTHandle{}
		}
		if positive {
			return c.hold(c.u.manager.BitVectorNumeral(value, width+n))
		}
	}
	return c.hold(c.u.manager.MkBitVectorSignExtend(n, h))
}

func (c *bv2RealCall) signedReal(h ast.ASTHandle) (result ast.ASTHandle) {
	mark := c.checkpoint()
	defer c.endExpressionScope(mark, &result)
	width := c.width(h)
	if c.err != nil {
		return ast.ASTHandle{}
	}
	// Source computes bsize before constructing either branch's AST nodes.
	power := c.rationalPowerTwo(width)
	if c.err != nil {
		return ast.ASTHandle{}
	}
	v := c.hold(c.u.manager.MkBitVectorToInt(h))
	v = c.arith(14, v)
	nonnegative := c.bv(23, c.numeral(big.NewRat(0, 1), width), h)
	if c.err != nil {
		return ast.ASTHandle{}
	}
	powerNumeral := c.hold(c.u.manager.ArithmeticNumeral(power, false))
	negative := c.arith(7, v, powerNumeral)
	return c.basic(ast.ASTOpIte, nonnegative, v, negative)
}
