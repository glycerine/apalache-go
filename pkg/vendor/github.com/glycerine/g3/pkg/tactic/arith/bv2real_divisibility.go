package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp:298-343.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (c *bv2RealCall) divisible(h ast.ASTHandle, overflow *big.Rat) (ast.ASTHandle, bool) {
	if c.err != nil {
		return h, false
	}
	count := len(c.numbers)
	defer func() { c.err = errors.Join(c.err, c.releaseNumbersTo(count)) }()
	input := c.rational(overflow)
	if c.err != nil {
		return h, false
	}
	return c.divisibleOwned(h, input)
}

// The live cap path passes the actual project overflow owner directly, just
// as the source const rational& parameter; only the local copy is owned here.
func (c *bv2RealCall) divisibleOwned(h ast.ASTHandle, overflow *unumeral.OwnedRational) (ast.ASTHandle, bool) {
	if c.err != nil {
		return h, false
	}
	count := len(c.numbers)
	defer func() { c.err = errors.Join(c.err, c.releaseNumbersTo(count)) }()
	s := c.u.manager.NumeralService()
	rest := c.number()
	if c.err != nil {
		return h, false
	}
	if c.err = s.Copy(rest, overflow); c.err != nil {
		return h, false
	}
	integer, err := s.IsInt(rest)
	if err != nil {
		c.err = err
		return h, false
	}
	positive, err := s.IsPos(rest)
	if err != nil {
		c.err = err
		return h, false
	}
	one, err := s.IsOne(rest)
	if err != nil {
		c.err = err
		return h, false
	}
	for _, check := range []struct {
		line int32
		text string
		ok   bool
	}{
		{315, "overflow.is_int()", integer},
		{316, "overflow.is_pos()", positive},
		{317, "!overflow.is_one()", !one},
	} {
		if c.err = bv2RealSourceAssert(check.line, check.text, check.ok); c.err != nil {
			return h, false
		}
	}
	if !integer || !positive || one {
		c.err = ast.ErrASTDomain
		return h, false
	}
	if c.err = c.u.traceDivisible(h, rest, 318); c.err != nil {
		return h, false
	}
	two := c.number()
	if c.err == nil {
		c.err = s.SetInt64(two, 2)
	}
	if c.err != nil {
		return h, false
	}
	var power uint32
	for {
		local := len(c.numbers)
		rem := c.takeRational(s.Rem(rest, two))
		if c.err != nil {
			return h, false
		}
		var zero bool
		zero, c.err = s.IsZero(rem)
		c.err = errors.Join(c.err, c.releaseNumbersTo(local))
		if c.err != nil {
			return h, false
		}
		if !zero {
			break
		}
		power++
		quotient := c.takeRational(s.IDiv(rest, two))
		if c.err != nil {
			return h, false
		}
		c.err = s.Copy(rest, quotient)
		c.err = errors.Join(c.err, c.releaseNumbersTo(local))
		if c.err != nil {
			return h, false
		}
	}
	if power > 0 {
		width := c.width(h)
		if c.err != nil {
			return h, false
		}
		if width <= power {
			c.side(c.basic(ast.ASTOpEq, h, c.numeral(big.NewRat(0, 1), width)))
			if c.err != nil {
				return h, false
			}
			next := c.numeral(big.NewRat(0, 1), 1)
			if c.err != nil {
				return h, false
			}
			h = next
		} else {
			low := c.extract(power-1, 0, h)
			c.side(c.basic(ast.ASTOpEq, low, c.numeral(big.NewRat(0, 1), power)))
			if c.err != nil {
				return h, false
			}
			next := c.extract(width-1, power, h)
			if c.err != nil {
				return h, false
			}
			h = next
		}
	}
	if c.err = c.u.traceDivisible(h, rest, 339); c.err != nil {
		return h, false
	}
	one, err = s.IsOne(rest)
	c.err = err
	return h, one && err == nil
}

func (c *bv2RealCall) alignDivisor(a, b ast.ASTHandle, d *big.Rat) (ast.ASTHandle, ast.ASTHandle, bool) {
	if c.err != nil {
		return a, b, false
	}
	maximum := c.rationalValue(&c.u.defaults[2])
	if c.err != nil {
		return a, b, false
	}
	if d.Cmp(maximum) <= 0 {
		return a, b, true
	}
	if maximum.Sign() == 0 {
		c.err = ast.ErrASTDomain
		return a, b, false
	}
	count := len(c.numbers)
	defer func() { c.err = errors.Join(c.err, c.releaseNumbersTo(count)) }()
	s := c.u.manager.NumeralService()
	input := c.rational(d)
	if c.err != nil {
		return a, b, false
	}
	overflow := c.takeRational(s.Div(input, &c.u.defaults[2]))
	if c.err != nil {
		return a, b, false
	}
	integral, err := s.IsInt(overflow)
	c.err = err
	if err != nil || !integral {
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
	if c.err = s.Copy(input, &c.u.defaults[2]); c.err != nil {
		return a, b, false
	}
	capped := c.rationalValue(input)
	if c.err != nil {
		return a, b, false
	}
	d.Set(capped)
	return a, b, true
}
