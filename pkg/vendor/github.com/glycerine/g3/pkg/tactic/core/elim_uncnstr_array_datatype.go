package core

// Source: src/tactic/core/elim_uncnstr_tactic.cpp array/datatype reductions.

import "github.com/glycerine/g3/pkg/ast"

func (c *elimUnconstrainedRewriteConfig) arrayApp(d ast.ASTHandle, kind int32, args []ast.ASTHandle) (ast.ASTHandle, error) {
	m := c.owner.manager
	switch kind {
	case 1: // OP_SELECT
		if !c.owner.variables[args[0]] {
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
		definition, err := c.keep(m.MkConstArray(s, u))
		if err != nil {
			return u, err
		}
		return u, c.addDef(args[0], definition)
	case 0: // OP_STORE
		last := len(args) - 1
		if !c.owner.variables[args[0]] || !c.owner.variables[args[last]] {
			return ast.ASTHandle{}, nil
		}
		u, fresh, err := c.freshFor(d, args)
		if err != nil || !fresh || c.converter == nil {
			return u, err
		}
		// Preserve the source definition order and select from the original
		// array. Reverse model reconstruction resolves its later definition.
		definition, err := c.app(c.owner.array, 1, args[:last]...)
		if err != nil {
			return u, err
		}
		if err = c.addDef(args[last], definition); err != nil {
			return u, err
		}
		return u, c.addDef(args[0], u)
	}
	return ast.ASTHandle{}, nil
}

func (c *elimUnconstrainedRewriteConfig) datatypeApp(d ast.ASTHandle, kind int32, args []ast.ASTHandle) (ast.ASTHandle, error) {
	m := c.owner.manager
	switch kind {
	case ast.ASTDatatypeRecognizer, ast.ASTDatatypeIs:
		if c.owner.variables[args[0]] && c.converter == nil {
			u, _, err := c.freshFor(d, args)
			return u, err
		}
		// Source deliberately does not eliminate recognizers when a model is
		// required: its model-generation branch is still a TODO.
	case ast.ASTDatatypeAccessor:
		if !c.owner.variables[args[0]] {
			return ast.ASTHandle{}, nil
		}
		if c.converter == nil {
			u, _, err := c.freshFor(d, args)
			return u, err
		}
		constructor, found, err := m.DatatypeAccessorConstructor(d)
		if err != nil || !found {
			return ast.ASTHandle{}, err
		}
		arity, err := m.GetArity(constructor)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		for i := uint32(0); i < arity; i++ {
			domain, e := m.GetDomain(constructor, i)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			full, e := m.IsFullyInterpreted(domain)
			if e != nil || !full {
				return ast.ASTHandle{}, e
			}
		}
		u, fresh, err := c.freshFor(d, args)
		if err != nil || !fresh {
			return u, err
		}
		accessors, err := m.DatatypeConstructorAccessors(constructor)
		if err != nil {
			return u, err
		}
		values := make([]ast.ASTHandle, len(accessors))
		for i, accessor := range accessors {
			if accessor == d {
				values[i] = u
				continue
			}
			domain, e := m.GetDomain(constructor, uint32(i))
			if e != nil {
				return u, e
			}
			values[i], e = c.keep(m.GetSomeValue(domain))
			if e != nil {
				return u, e
			}
		}
		definition, err := c.apply(constructor, values)
		if err != nil {
			return u, err
		}
		return u, c.addDef(args[0], definition)
	case ast.ASTDatatypeConstructor:
		if !c.all(args) {
			return ast.ASTHandle{}, nil
		}
		u, fresh, err := c.freshFor(d, args)
		if err != nil || !fresh || c.converter == nil {
			return u, err
		}
		accessors, err := m.DatatypeConstructorAccessors(d)
		if err != nil {
			return u, err
		}
		for i, a := range args {
			definition, e := c.apply(accessors[i], []ast.ASTHandle{u})
			if e != nil {
				return u, e
			}
			if err = c.addDef(a, definition); err != nil {
				return u, err
			}
		}
		return u, nil
	}
	return ast.ASTHandle{}, nil
}
