package arith

// Source: src/tactic/arith/nla2bv_tactic.cpp:288-309.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Source: src/tactic/arith/nla2bv_tactic.cpp:225. All full-expression
// temporaries remain owned until log2 returns; caller bounds stay borrowed.
func (c *nla2BVInvocation) boundedBits(lower, upper *unumeral.OwnedRational) (bits uint32) {
	local := c.local
	if local.err != nil {
		return 0
	}
	count := len(local.numbers)
	defer func() { local.err = errors.Join(local.err, local.releaseNumbersTo(count)) }()
	s := c.t.manager.NumeralService()
	difference := local.takeRational(s.Sub(upper, lower))
	if local.err != nil {
		return 0
	}
	absolute := local.takeRational(s.Abs(difference))
	if local.err != nil {
		return 0
	}
	one := local.number()
	if local.err == nil {
		local.err = s.SetInt64(one, 1)
	}
	if local.err != nil {
		return 0
	}
	span := local.takeRational(s.Add(absolute, one))
	if local.err != nil {
		return 0
	}
	return c.log2Owned(span)
}

func (c *nla2BVInvocation) updateNumBits(h ast.ASTHandle) (err error) {
	local := c.local
	if local.err != nil {
		return local.err
	}
	count := len(local.numbers)
	number := &bv2RealArithmeticRead{manager: c.t.manager}
	defer func() {
		// Failed copy invocations can still borrow the absolute-value temporary.
		// Retire their lower roots before releasing those enclosing owners.
		if cleanup := number.close(); cleanup != nil {
			local.readPending = append(local.readPending, number)
			err = errors.Join(err, cleanup)
		} else {
			err = errors.Join(err, local.releaseNumbersTo(count))
		}
		local.err = errors.Join(local.err, err)
		err = local.err
	}()
	s := c.t.manager.NumeralService()
	number.value, err = s.NewValue()
	number.ready = err == nil
	if err != nil {
		return err
	}
	integer := false
	numeral, err := c.t.manager.ArithmeticNumeralValueWithInvocation(c.real.arith, h, &number.value, &integer, &number.invocation)
	if err == nil {
		err = number.retire()
	}
	if err != nil || !numeral || !integer {
		return err
	}
	absolute := local.takeRational(s.Abs(&number.value))
	if local.err != nil {
		return local.err
	}
	if err = s.CopyWithInvocation(&number.value, absolute, &number.invocation); err != nil {
		return err
	}
	if err = number.retire(); err != nil {
		return err
	}
	if err = local.releaseNumbersTo(count); err != nil {
		return err
	}
	bits := c.log2Owned(&number.value)
	if local.err != nil {
		return local.err
	}
	if c.numBits <= bits {
		c.numBits = bits + 1
	}
	return nil
}

// Keep the actual source pow receiver alive across compound multiplications.
// The same concrete numeric invocation owner used for arithmetic numeral reads
// holds failed lower-operation roots until the receiver is destroyed.
func (c *nla2BVInvocation) log2Owned(n *unumeral.OwnedRational) (bits uint32) {
	local := c.local
	if local.err != nil {
		return 0
	}
	count := len(local.numbers)
	pow := &bv2RealArithmeticRead{manager: c.t.manager}
	defer func() {
		local.err = errors.Join(local.err, local.releaseNumbersTo(count))
		if err := pow.close(); err != nil {
			local.readPending = append(local.readPending, pow)
			local.err = errors.Join(local.err, err)
		}
	}()
	s := c.t.manager.NumeralService()
	pow.value, local.err = s.NewValue()
	pow.ready = local.err == nil
	if local.err != nil {
		return 0
	}
	local.err = s.SetInt64(&pow.value, 1)
	two := local.number()
	if local.err == nil {
		local.err = s.SetInt64(two, 2)
	}
	for local.err == nil {
		var less bool
		less, local.err = s.Less(&pow.value, n)
		if local.err != nil || !less {
			break
		}
		bits++
		local.err = s.MulAssignWithInvocation(&pow.value, two, &pow.invocation)
		if local.err == nil {
			local.err = pow.retire()
		}
	}
	if bits == 0 && local.err == nil {
		bits = 1
	}
	return bits
}
