package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp:25-37,210-212.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (u *BV2RealUtil) initializeDefaults(root, divisor uint32) (err error) {
	s := u.manager.NumeralService()
	for i, n := range []uint32{root, divisor} {
		u.defaults[i], err = s.NewValue()
		if err != nil {
			return err
		}
		if err = s.SetUnsigned(&u.defaults[i], n); err != nil {
			return err
		}
	}
	call := &bv2RealCall{u: u}
	defer u.finish(call, &err)
	two := call.number()
	if call.err != nil {
		return call.err
	}
	if err = s.SetInt64(two, 2); err != nil {
		return err
	}
	u.defaults[2], err = s.Mul(two, &u.defaults[1])
	return err
}

func (c *bv2RealCall) isZero(h ast.ASTHandle) bool {
	if c.err != nil {
		return false
	}
	count := len(c.numbers)
	defer func() { c.err = errors.Join(c.err, c.releaseNumbersTo(count)) }()
	value := c.number()
	if c.err != nil {
		return false
	}
	var width uint32
	numeral, err := c.u.manager.BitVectorNumeralValue(h, value, &width)
	if err != nil {
		c.err = err
		return false
	}
	if !numeral {
		return false
	}
	zero, err := c.u.manager.NumeralService().IsZero(value)
	c.err = err
	return zero && err == nil
}
