package core

// Source: src/tactic/core/elim_uncnstr_tactic.cpp bit-vector reductions.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *elimUnconstrainedRewriteConfig) bv(d ast.ASTHandle, kind int32, args []ast.ASTHandle) (ast.ASTHandle, error) {
	m := c.owner.manager
	switch kind {
	case 4: // OP_BADD
		return c.addition(d, c.owner.bitVector, 4, 5, args)
	case 6: // OP_BMUL
		return c.bvMultiply(d, args)
	case 7, 8, 17, 18: // signed/unsigned division, including internal operators
		if !c.all(args) {
			return ast.ASTHandle{}, nil
		}
		u, fresh, err := c.freshFor(d, args)
		if err != nil || !fresh || c.converter == nil {
			return u, err
		}
		s, err := m.GetSort(args[0])
		if err != nil {
			return u, err
		}
		one, err := c.numeral(s, 1)
		if err != nil {
			return u, err
		}
		if err = c.addDef(args[0], u); err != nil {
			return u, err
		}
		return u, c.addDef(args[1], one)
	case 22, 23: // OP_ULEQ, OP_SLEQ
		return c.bvLessEqual(d, args, kind == 23)
	case 37: // OP_CONCAT
		if !c.all(args) {
			return ast.ASTHandle{}, nil
		}
		u, fresh, err := c.freshFor(d, args)
		if err != nil || !fresh || c.converter == nil {
			return u, err
		}
		low := uint32(0)
		for i := len(args) - 1; i >= 0; i-- {
			width, e := m.BitVectorSize(args[i])
			if e != nil {
				return u, e
			}
			part, e := c.keep(m.MkBitVectorExtract(low+width-1, low, u))
			if e != nil {
				return u, e
			}
			if err = c.addDef(args[i], part); err != nil {
				return u, err
			}
			low += width
		}
		return u, nil
	case 40: // OP_EXTRACT
		if !c.owner.variables[args[0]] {
			return ast.ASTHandle{}, nil
		}
		u, fresh, err := c.freshFor(d, args)
		if err != nil || !fresh || c.converter == nil {
			return u, err
		}
		original, err := c.apply(d, args)
		if err != nil {
			return u, err
		}
		high, err := m.BitVectorExtractHigh(original)
		if err != nil {
			return u, err
		}
		low, err := m.BitVectorExtractLow(original)
		if err != nil {
			return u, err
		}
		width, err := m.BitVectorSize(args[0])
		if err != nil {
			return u, err
		}
		definition := u
		if width != high-low+1 {
			var parts []ast.ASTHandle
			if high < width-1 {
				zero, e := c.keep(m.BitVectorNumeralUint64(0, width-high-1))
				if e != nil {
					return u, e
				}
				parts = append(parts, zero)
			}
			parts = append(parts, u)
			if low > 0 {
				zero, e := c.keep(m.BitVectorNumeralUint64(0, low))
				if e != nil {
					return u, e
				}
				parts = append(parts, zero)
			}
			definition, err = c.app(c.owner.bitVector, 37, parts...)
			if err != nil {
				return u, err
			}
		}
		return u, c.addDef(args[0], definition)
	case 32: // OP_BNOT
		if !c.owner.variables[args[0]] {
			return ast.ASTHandle{}, nil
		}
		u, fresh, err := c.freshFor(d, args)
		if err != nil || !fresh || c.converter == nil {
			return u, err
		}
		definition, err := c.apply(d, []ast.ASTHandle{u})
		if err != nil {
			return u, err
		}
		return u, c.addDef(args[0], definition)
	case 31: // OP_BOR
		if !c.all(args) {
			return ast.ASTHandle{}, nil
		}
		u, fresh, err := c.freshFor(d, args)
		if err != nil || !fresh || c.converter == nil {
			return u, err
		}
		s, err := m.GetSort(args[0])
		if err != nil {
			return u, err
		}
		zero, err := c.numeral(s, 0)
		if err != nil {
			return u, err
		}
		return u, c.addDefs(args, u, zero)
	}
	return ast.ASTHandle{}, nil
}

func (c *elimUnconstrainedRewriteConfig) bvMultiply(d ast.ASTHandle, args []ast.ASTHandle) (u ast.ASTHandle, err error) {
	m := c.owner.manager
	if c.all(args) {
		var fresh bool
		u, fresh, err = c.freshFor(d, args)
		if err != nil || !fresh || c.converter == nil {
			return u, err
		}
		s, e := m.GetSort(args[0])
		if e != nil {
			return u, e
		}
		one, e := c.numeral(s, 1)
		if e != nil {
			return u, e
		}
		return u, c.addDefs(args, u, one)
	}
	if len(args) != 2 || !c.owner.variables[args[1]] {
		return u, nil
	}
	numbers := m.NumeralService()
	n, err := numbers.One()
	if err != nil {
		return u, err
	}
	defer func() { err = errors.Join(err, numbers.Del(&n)) }()
	width := uint32(0)
	numeral, err := m.BitVectorNumeralValue(args[0], &n, &width)
	if err != nil || !numeral {
		return u, err
	}
	inverse, err := numbers.One()
	if err != nil {
		return u, err
	}
	defer func() { err = errors.Join(err, numbers.Del(&inverse)) }()
	invertible, err := m.BitVectorMultiplicativeInverse(&n, width, &inverse)
	if err != nil || !invertible {
		return u, err
	}
	u, fresh, err := c.freshFor(d, args)
	if err != nil || !fresh || c.converter == nil {
		return u, err
	}
	coefficient, err := c.keep(m.BitVectorNumeral(&inverse, width))
	if err != nil {
		return u, err
	}
	definition, err := c.app(c.owner.bitVector, 6, coefficient, u)
	if err != nil {
		return u, err
	}
	return u, c.addDef(args[1], definition)
}

func (c *elimUnconstrainedRewriteConfig) bvLessEqual(d ast.ASTHandle, args []ast.ASTHandle, signed bool) (u ast.ASTHandle, err error) {
	if c.proofs {
		return u, nil
	} // Source has no proof rule for the side condition.
	first := c.owner.variables[args[0]]
	v, t := args[0], args[1]
	if !first {
		v, t = t, v
		if !c.owner.variables[v] {
			return u, nil
		}
	}
	m := c.owner.manager
	width, err := m.BitVectorSize(args[0])
	if err != nil {
		return u, err
	}
	numbers := m.NumeralService()
	bound, err := numbers.One()
	if err != nil {
		return u, err
	}
	defer func() { err = errors.Join(err, numbers.Del(&bound)) }()
	if first || signed {
		power := width
		if signed {
			power--
		}
		value, e := numbers.PowerOfTwo(power)
		if e != nil {
			return u, e
		}
		e = numbers.Copy(&bound, &value)
		if e = errors.Join(e, numbers.Del(&value)); e != nil {
			return u, e
		}
		if first {
			one, e := numbers.One()
			if e != nil {
				return u, e
			}
			e = numbers.SubAssign(&bound, &one)
			if e = errors.Join(e, numbers.Del(&one)); e != nil {
				return u, e
			}
		} else {
			if e := numbers.NegAssign(&bound); e != nil {
				return u, e
			}
		}
	} else {
		if err = numbers.SetInt64(&bound, 0); err != nil {
			return u, err
		}
	}
	u, fresh, err := c.freshFor(d, args)
	if err != nil {
		return u, err
	}
	limit, err := c.keep(m.BitVectorNumeral(&bound, width))
	if err != nil {
		return u, err
	}
	eq, err := c.keep(m.MkEq(t, limit))
	if err != nil {
		return u, err
	}
	result, err := c.keep(m.MkOr(u, eq))
	if err != nil || c.converter == nil || !fresh {
		return result, err
	}
	sort, err := m.GetSort(args[0])
	if err != nil {
		return result, err
	}
	one, err := c.numeral(sort, 1)
	if err != nil {
		return result, err
	}
	op := int32(5)
	if first {
		op = 4
	}
	diff, err := c.app(c.owner.bitVector, op, t, one)
	if err != nil {
		return result, err
	}
	definition, err := c.keep(m.MkIte(result, t, diff))
	if err != nil {
		return result, err
	}
	return result, c.addDef(v, definition)
}
