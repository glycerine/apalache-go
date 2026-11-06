package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp:159-176.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// rational imports a caller-facing value into the concrete project owner.
func (c *bv2RealCall) rational(q *big.Rat) *unumeral.OwnedRational {
	v := c.number()
	if v != nil {
		c.err = c.u.manager.NumeralService().SetString(v, q.RatString())
	}
	if c.err != nil {
		return nil
	}
	return v
}

// takeRational accepts ownership of a project arithmetic return, including
// any nonzero owner returned with an error so cleanup never loses it.
func (c *bv2RealCall) takeRational(v unumeral.OwnedRational, err error) *unumeral.OwnedRational {
	if v != (unumeral.OwnedRational{}) {
		c.numbers = append(c.numbers, &v)
	}
	c.err = errors.Join(c.err, err)
	if c.err != nil {
		return nil
	}
	if v == (unumeral.OwnedRational{}) {
		c.err = ast.ErrASTDomain
		return nil
	}
	return &v
}

func (c *bv2RealCall) rationalValue(v *unumeral.OwnedRational) *big.Rat {
	if c.err != nil {
		return nil
	}
	text, err := c.u.manager.NumeralService().String(v)
	if err != nil {
		c.err = err
		return nil
	}
	q, ok := new(big.Rat).SetString(text)
	if !ok {
		c.err = ast.ErrASTDomain
	}
	return q
}

func (c *bv2RealCall) alignDivisors(a, b, x, y ast.ASTHandle, d1, d2 *big.Rat) (ast.ASTHandle, ast.ASTHandle, ast.ASTHandle, ast.ASTHandle) {
	if c.err != nil || d1.Cmp(d2) == 0 {
		return a, b, x, y
	}
	if !d1.IsInt() || !d2.IsInt() {
		c.err = ast.ErrASTDomain
		return a, b, x, y
	}
	count := len(c.numbers)
	defer func() { c.err = errors.Join(c.err, c.releaseNumbersTo(count)) }()
	s := c.u.manager.NumeralService()
	first := c.rational(d1)
	second := c.rational(d2)
	if c.err != nil {
		return a, b, x, y
	}
	g := c.takeRational(s.GCD(first, second))
	if c.err != nil {
		return a, b, x, y
	}
	l := c.takeRational(s.LCM(first, second))
	if c.err != nil {
		return a, b, x, y
	}
	d1g := c.takeRational(s.Div(first, g))
	if c.err != nil {
		return a, b, x, y
	}
	d2g := c.takeRational(s.Div(second, g))
	if c.err != nil {
		return a, b, x, y
	}
	left, right := c.rationalValue(d2g), c.rationalValue(d1g)
	if c.err != nil {
		return a, b, x, y
	}
	// Each expr_ref assignment completes before the next scaling operation.
	next := c.multiplyScalar(left, a)
	if c.err != nil {
		return a, b, x, y
	}
	a = next
	next = c.multiplyScalar(left, b)
	if c.err != nil {
		return a, b, x, y
	}
	b = next
	next = c.multiplyScalar(right, x)
	if c.err != nil {
		return a, b, x, y
	}
	x = next
	next = c.multiplyScalar(right, y)
	if c.err != nil {
		return a, b, x, y
	}
	y = next
	// The source copies the common divisor only after all four assignments.
	if c.err = s.Copy(first, l); c.err != nil {
		return a, b, x, y
	}
	common := c.rationalValue(first)
	if c.err != nil {
		return a, b, x, y
	}
	d1.Set(common)
	if c.err = s.Copy(second, l); c.err != nil {
		return a, b, x, y
	}
	common = c.rationalValue(second)
	if c.err == nil {
		d2.Set(common)
	}
	return a, b, x, y
}
