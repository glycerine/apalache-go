package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp:127-141,153-176,298-310.
// Concrete project-rational operands used by live reducer calls.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (c *bv2RealCall) multiplyScalarOwned(n *unumeral.OwnedRational, b ast.ASTHandle) (result ast.ASTHandle) {
	mark := c.checkpoint()
	defer c.endExpressionScope(mark, &result)
	if c.err != nil {
		return ast.ASTHandle{}
	}
	one, err := c.u.manager.NumeralService().IsOne(n)
	if err != nil {
		c.err = err
		return ast.ASTHandle{}
	}
	if one {
		return b
	}
	return c.mul(c.signedNumeralOwned(n), b)
}
func (c *bv2RealCall) multiplyScalarLiteral(n int64, b ast.ASTHandle) (result ast.ASTHandle) {
	mark := c.checkpoint()
	defer c.endExpressionScope(mark, &result)
	value := c.number()
	if c.err == nil {
		c.err = c.u.manager.NumeralService().SetInt64(value, n)
	}
	if c.err != nil {
		return ast.ASTHandle{}
	}
	return c.multiplyScalarOwned(value, b)
}

func (c *bv2RealCall) alignDivisorsOwned(a, b, x, y ast.ASTHandle, d1, d2 *unumeral.OwnedRational) (ast.ASTHandle, ast.ASTHandle, ast.ASTHandle, ast.ASTHandle) {
	if c.err != nil {
		return a, b, x, y
	}
	s := c.u.manager.NumeralService()
	equal, err := s.Equal(d1, d2)
	if err != nil {
		c.err = err
		return a, b, x, y
	}
	if equal {
		return a, b, x, y
	}
	count := len(c.numbers)
	defer func() { c.err = errors.Join(c.err, c.releaseNumbersTo(count)) }()
	g := c.takeRational(s.GCD(d1, d2))
	if c.err != nil {
		return a, b, x, y
	}
	l := c.takeRational(s.LCM(d1, d2))
	if c.err != nil {
		return a, b, x, y
	}
	d1g := c.takeRational(s.Div(d1, g))
	if c.err != nil {
		return a, b, x, y
	}
	d2g := c.takeRational(s.Div(d2, g))
	if c.err != nil {
		return a, b, x, y
	}
	next := c.multiplyScalarOwned(d2g, a)
	if c.err != nil {
		return a, b, x, y
	}
	a = next
	next = c.multiplyScalarOwned(d2g, b)
	if c.err != nil {
		return a, b, x, y
	}
	b = next
	next = c.multiplyScalarOwned(d1g, x)
	if c.err != nil {
		return a, b, x, y
	}
	x = next
	next = c.multiplyScalarOwned(d1g, y)
	if c.err != nil {
		return a, b, x, y
	}
	y = next
	if c.err = s.Copy(d1, l); c.err != nil {
		return a, b, x, y
	}
	c.err = s.Copy(d2, l)
	return a, b, x, y
}

func (c *bv2RealCall) alignDivisorOwned(a, b ast.ASTHandle, d *unumeral.OwnedRational) (ast.ASTHandle, ast.ASTHandle, bool) {
	if c.err != nil {
		return a, b, false
	}
	s := c.u.manager.NumeralService()
	larger, err := s.Greater(d, &c.u.defaults[2])
	if err != nil {
		c.err = err
		return a, b, false
	}
	if !larger {
		return a, b, true
	}
	count := len(c.numbers)
	defer func() { c.err = errors.Join(c.err, c.releaseNumbersTo(count)) }()
	overflow := c.takeRational(s.Div(d, &c.u.defaults[2]))
	if c.err != nil {
		return a, b, false
	}
	integer, err := s.IsInt(overflow)
	if err != nil {
		c.err = err
		return a, b, false
	}
	if !integer {
		return a, b, false
	}
	var ok bool
	a, ok = c.divisibleOwned(a, overflow)
	if c.err != nil || !ok {
		return a, b, false
	}
	b, ok = c.divisibleOwned(b, overflow)
	if c.err != nil || !ok {
		return a, b, false
	}
	c.err = s.Copy(d, &c.u.defaults[2])
	return a, b, c.err == nil
}

func (c *bv2RealCall) reducedOwned(a, b ast.ASTHandle, d, r *unumeral.OwnedRational) (result ast.ASTHandle) {
	mark := c.checkpoint()
	defer c.endExpressionScope(mark, &result)
	x, y := c.signedReal(a), c.signedReal(b)
	if c.err != nil {
		return ast.ASTHandle{}
	}
	x = c.divideRealOwned(x, d)
	if c.err != nil {
		return ast.ASTHandle{}
	}
	y = c.divideRealOwned(y, d)
	if c.err != nil {
		return ast.ASTHandle{}
	}
	rn := c.hold(c.u.manager.ArithmeticNumeral(r, false))
	half := c.number()
	if c.err == nil {
		c.err = c.u.manager.NumeralService().SetFractionSigned(half, 1, 2)
	}
	if c.err != nil {
		return ast.ASTHandle{}
	}
	hn := c.hold(c.u.manager.ArithmeticNumeral(half, false))
	root := c.arith(18, rn, hn)
	y = c.arith(9, y, root)
	return c.arith(6, x, y)
}

// mk_div explicitly constructs rational(d) before passing it to mk_numeral.
// Its numeric copy and numeral reference end when the division is handed back.
func (c *bv2RealCall) divideRealOwned(e ast.ASTHandle, d *unumeral.OwnedRational) (result ast.ASTHandle) {
	mark := c.checkpoint()
	defer c.endExpressionScope(mark, &result)
	value := c.number()
	if c.err != nil {
		return ast.ASTHandle{}
	}
	if c.err = c.u.manager.NumeralService().Copy(value, d); c.err != nil {
		return ast.ASTHandle{}
	}
	numeral := c.hold(c.u.manager.ArithmeticNumeral(value, false))
	return c.arith(10, e, numeral)
}
