package core

// Source: src/tactic/core/elim_uncnstr_tactic.cpp basic and arithmetic reductions.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *elimUnconstrainedRewriteConfig) numeral(sort ast.ASTHandle, value int64) (h ast.ASTHandle, err error) {
	m := c.owner.manager
	numbers := m.NumeralService()
	n, err := numbers.One()
	if err != nil {
		return h, err
	}
	defer func() { err = errors.Join(err, numbers.Del(&n)) }()
	if err = numbers.SetInt64(&n, value); err != nil {
		return h, err
	}
	fid, err := m.GetFamilyID(sort)
	if err != nil {
		return h, err
	}
	if fid == c.owner.bitVector {
		return c.keep(m.BitVectorNumeralForSort(&n, sort))
	}
	kind, err := m.GetDeclKind(sort)
	if err != nil {
		return h, err
	}
	return c.keep(m.ArithmeticNumeral(&n, kind == 1))
}

func (c *elimUnconstrainedRewriteConfig) basic(d ast.ASTHandle, kind int32, args []ast.ASTHandle) (ast.ASTHandle, error) {
	m := c.owner.manager
	switch kind {
	case ast.ASTOpIte:
		first, second := -1, -1
		var identity ast.ASTHandle
		if c.owner.variables[args[1]] && c.owner.variables[args[2]] {
			first, second = 1, 2
		} else if c.owner.variables[args[0]] && c.owner.variables[args[1]] {
			first, second = 0, 1
			var err error
			identity, err = c.keep(m.MkTrue())
			if err != nil {
				return identity, err
			}
		} else if c.owner.variables[args[0]] && c.owner.variables[args[2]] {
			first, second = 0, 2
			var err error
			identity, err = c.keep(m.MkFalse())
			if err != nil {
				return identity, err
			}
		}
		if first < 0 {
			return ast.ASTHandle{}, nil
		}
		u, fresh, err := c.freshFor(d, args)
		if err != nil || !fresh {
			return u, err
		}
		if identity.IsNull() {
			identity = u
		}
		if err = c.addDef(args[first], identity); err != nil {
			return u, err
		}
		return u, c.addDef(args[second], u)
	case ast.ASTOpNot:
		if !c.owner.variables[args[0]] {
			return ast.ASTHandle{}, nil
		}
		u, fresh, err := c.freshFor(d, args)
		if err != nil || !fresh {
			return u, err
		}
		if c.converter != nil {
			value, e := c.keep(m.MkNot(u))
			if e != nil {
				return u, e
			}
			err = c.addDef(args[0], value)
		}
		return u, err
	case ast.ASTOpAnd, ast.ASTOpOr:
		if !c.all(args) {
			return ast.ASTHandle{}, nil
		}
		u, fresh, err := c.freshFor(d, args)
		if err != nil || !fresh || c.converter == nil {
			return u, err
		}
		var identity ast.ASTHandle
		if kind == ast.ASTOpAnd {
			identity, err = c.keep(m.MkTrue())
		} else {
			identity, err = c.keep(m.MkFalse())
		}
		if err != nil {
			return u, err
		}
		return u, c.addDefs(args, u, identity)
	case ast.ASTOpEq, ast.ASTOpIff:
		return c.equality(d, args)
	}
	return ast.ASTHandle{}, nil
}

func (c *elimUnconstrainedRewriteConfig) equality(d ast.ASTHandle, args []ast.ASTHandle) (ast.ASTHandle, error) {
	v, t := args[0], args[1]
	if !c.owner.variables[v] {
		v, t = t, v
		if !c.owner.variables[v] {
			return ast.ASTHandle{}, nil
		}
	}
	m := c.owner.manager
	s, err := m.GetSort(args[0])
	if err != nil {
		return ast.ASTHandle{}, err
	}
	full, err := m.IsFullyInterpreted(s)
	if err != nil || !full {
		return ast.ASTHandle{}, err
	}
	size, err := m.GetSortSize(s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if size.IsFinite() {
		n, e := size.Size()
		if e != nil || n <= 1 {
			return ast.ASTHandle{}, e
		}
	}
	if c.converter == nil {
		u, _, e := c.freshFor(d, args)
		return u, e
	}
	diff, found, err := c.different(t)
	if err != nil || !found {
		return ast.ASTHandle{}, err
	}
	u, fresh, err := c.freshFor(d, args)
	if err != nil || !fresh {
		return u, err
	}
	definition, err := c.keep(m.MkIte(u, t, diff))
	if err != nil {
		return u, err
	}
	return u, c.addDef(v, definition)
}

func (c *elimUnconstrainedRewriteConfig) different(t ast.ASTHandle) (ast.ASTHandle, bool, error) {
	m := c.owner.manager
	s, err := m.GetSort(t)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if s == boolean {
		h, e := c.keep(m.MkNot(t))
		return h, e == nil, e
	}
	fid, err := m.GetFamilyID(s)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	switch fid {
	case c.owner.arithmetic:
		one, e := c.numeral(s, 1)
		if e != nil {
			return one, false, e
		}
		h, e := c.keep(m.MkArithmeticAdd(t, one))
		return h, e == nil, e
	case c.owner.bitVector:
		h, e := c.app(fid, 32, t)
		return h, e == nil, e
	case c.owner.array:
		rangeSort, e := m.ArrayRange(s)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		uninterpreted, e := m.IsUninterpretedSort(rangeSort)
		if e != nil || uninterpreted {
			return ast.ASTHandle{}, false, e
		}
		arity, e := m.ArrayArity(s)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		for i := uint32(0); i < arity; i++ {
			domain, e := m.ArrayDomain(s, i)
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			uninterpreted, e = m.IsUninterpretedSort(domain)
			if e != nil || uninterpreted {
				return ast.ASTHandle{}, false, e
			}
		}
		args := []ast.ASTHandle{t}
		for i := uint32(0); i < arity; i++ {
			domain, e := m.ArrayDomain(s, i)
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			value, e := c.keep(m.GetSomeValue(domain))
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			args = append(args, value)
		}
		selected, e := c.app(fid, 1, args...)
		if e != nil {
			return selected, false, e
		}
		value, found, e := c.different(selected)
		if e != nil || !found {
			return ast.ASTHandle{}, false, e
		}
		args = append(args, value)
		h, e := c.app(fid, 0, args...)
		return h, e == nil, e
	case c.owner.datatype:
		constructors, e := m.DatatypeConstructors(s)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		for _, constructor := range constructors {
			arity, e := m.GetArity(constructor)
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			target := ^uint32(0)
			for i := uint32(0); i < arity; i++ {
				domain, e := m.GetDomain(constructor, i)
				if e != nil {
					return ast.ASTHandle{}, false, e
				}
				if domain == s {
					target = i
					continue
				}
				uninterpreted, e := m.IsUninterpretedSort(domain)
				if e != nil {
					return ast.ASTHandle{}, false, e
				}
				if uninterpreted {
					break
				}
			}
			if target == ^uint32(0) {
				continue
			}
			args := make([]ast.ASTHandle, arity)
			for i := range args {
				if uint32(i) == target {
					args[i] = t
					continue
				}
				domain, e := m.GetDomain(constructor, uint32(i))
				if e != nil {
					return ast.ASTHandle{}, false, e
				}
				args[i], e = c.keep(m.GetSomeValue(domain))
				if e != nil {
					return ast.ASTHandle{}, false, e
				}
			}
			h, e := c.apply(constructor, args)
			return h, e == nil, e
		}
	}
	return ast.ASTHandle{}, false, nil
}

func (c *elimUnconstrainedRewriteConfig) addition(d ast.ASTHandle, fid ast.FamilyID, add, sub int32, args []ast.ASTHandle) (ast.ASTHandle, error) {
	selected := -1
	for i, a := range args {
		if c.owner.variables[a] {
			selected = i
			break
		}
	}
	if selected < 0 {
		return ast.ASTHandle{}, nil
	}
	u, fresh, err := c.freshFor(d, args)
	if err != nil || !fresh || c.converter == nil {
		return u, err
	}
	rest := append([]ast.ASTHandle{}, args[:selected]...)
	rest = append(rest, args[selected+1:]...)
	definition := u
	if len(rest) != 0 {
		value := rest[0]
		if len(rest) > 1 {
			value, err = c.app(fid, add, rest...)
			if err != nil {
				return u, err
			}
		}
		definition, err = c.app(fid, sub, u, value)
		if err != nil {
			return u, err
		}
	}
	return u, c.addDef(args[selected], definition)
}

func (c *elimUnconstrainedRewriteConfig) arith(d ast.ASTHandle, kind int32, args []ast.ASTHandle) (result ast.ASTHandle, resultErr error) {
	m := c.owner.manager
	switch kind {
	case 6: // OP_ADD
		return c.addition(d, c.owner.arithmetic, 6, 7, args)
	case 9: // OP_MUL
		if c.all(args) {
			u, fresh, err := c.freshFor(d, args)
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
			return ast.ASTHandle{}, nil
		}
		text, integer, numeral, err := m.ArithmeticNumeralText(args[0])
		if err != nil || !numeral || integer {
			return ast.ASTHandle{}, err
		}
		numbers := m.NumeralService()
		value, err := numbers.One()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		defer func() { resultErr = errors.Join(resultErr, numbers.Del(&value)) }()
		if err = numbers.SetString(&value, text); err != nil {
			return ast.ASTHandle{}, err
		}
		zero, err := numbers.IsZero(&value)
		if err != nil || zero {
			return ast.ASTHandle{}, err
		}
		u, fresh, err := c.freshFor(d, args)
		if err != nil || !fresh || c.converter == nil {
			return u, err
		}
		one, err := numbers.One()
		if err != nil {
			return u, err
		}
		defer func() { resultErr = errors.Join(resultErr, numbers.Del(&one)) }()
		inverse, err := numbers.Div(&one, &value)
		if err != nil {
			return u, err
		}
		defer func() { resultErr = errors.Join(resultErr, numbers.Del(&inverse)) }()
		coefficient, err := c.keep(m.ArithmeticNumeral(&inverse, false))
		if err != nil {
			return u, err
		}
		definition, err := c.keep(m.MkArithmeticMul(coefficient, u))
		if err != nil {
			return u, err
		}
		return u, c.addDef(args[1], definition)
	case 2, 3: // OP_LE, OP_GE
		le := kind == 2
		v, t := args[0], args[1]
		if !c.owner.variables[v] {
			v, t = t, v
			le = !le
			if !c.owner.variables[v] {
				return ast.ASTHandle{}, nil
			}
		}
		u, fresh, err := c.freshFor(d, args)
		if err != nil || !fresh || c.converter == nil {
			return u, err
		}
		s, err := m.GetSort(args[0])
		if err != nil {
			return u, err
		}
		increment := int64(-1)
		if le {
			increment = 1
		}
		n, err := c.numeral(s, increment)
		if err != nil {
			return u, err
		}
		diff, err := c.keep(m.MkArithmeticAdd(t, n))
		if err != nil {
			return u, err
		}
		definition, err := c.keep(m.MkIte(u, t, diff))
		if err != nil {
			return u, err
		}
		return u, c.addDef(v, definition)
	}
	return ast.ASTHandle{}, nil
}
