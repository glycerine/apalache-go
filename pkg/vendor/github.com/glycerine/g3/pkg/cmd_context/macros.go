package cmd_context

// Source: src/cmd_context/cmd_context.cpp macro_decls, insert, macros_find,
// erase_macro and mk_app; src/ast/ast.cpp compatible_sorts.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type macroDeclaration struct {
	domain []ast.ASTHandle
	body   ast.ASTHandle
	refs   []ast.ASTRef
}

func sameDomain(a, b []ast.ASTHandle) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (c *Context) containsMacro(name string, domain []ast.ASTHandle) bool {
	for _, d := range c.macros[name] {
		if sameDomain(d.domain, domain) {
			return true
		}
	}
	return false
}

// DefineFunction installs a nonrecursive SMT-LIB definition. The parser has
// checked the declared result sort; variables in body use reversed binder order.
func (c *Context) DefineFunction(name string, domain []ast.ASTHandle, body ast.ASTHandle) error {
	return c.defineFunction(name, domain, body, false)
}

func (c *Context) defineFunction(name string, domain []ast.ASTHandle, body ast.ASTHandle, recursiveIdentity bool) error {
	m, err := c.Manager()
	if err != nil {
		return err
	}
	if len(c.builtins[name]) != 0 {
		return fmt.Errorf("invalid macro/named expression, builtin symbol %s", name)
	}
	if c.containsMacro(name, domain) {
		return fmt.Errorf("named expression already defined")
	}
	rangeSort, err := m.GetSort(body)
	if err != nil {
		return err
	}
	for _, d := range c.functions[name] {
		if recursiveIdentity {
			break
		}
		arity, err := m.GetArity(d)
		if err != nil {
			return err
		}
		r, err := m.GetRange(d)
		if err != nil {
			return err
		}
		if int(arity) != len(domain) || r != rangeSort {
			continue
		}
		matches := true
		for i, sort := range domain {
			actual, err := m.GetDomain(d, uint32(i))
			if err != nil {
				return err
			}
			if actual != sort {
				matches = false
				break
			}
		}
		if matches {
			return fmt.Errorf("invalid named expression, declaration already defined with this name %s", name)
		}
	}
	d := macroDeclaration{domain: append([]ast.ASTHandle(nil), domain...), body: body}
	// Pin unused parameter sorts too: unlike source borrowed domain pointers,
	// Go AST handles must remain valid even when the body omits a parameter.
	for _, h := range append(append([]ast.ASTHandle(nil), domain...), body) {
		ref, err := m.NewRef(h)
		if err != nil {
			for i := range d.refs {
				err = errors.Join(err, m.DelRef(&d.refs[i]))
			}
			return err
		}
		d.refs = append(d.refs, ref)
	}
	c.macros[name] = append(c.macros[name], d)
	c.macroTrail = append(c.macroTrail, name)
	return nil
}

func (c *Context) restoreMacros(size int) error {
	for len(c.macroTrail) > size {
		i := len(c.macroTrail) - 1
		name := c.macroTrail[i]
		values := c.macros[name]
		d := &values[len(values)-1]
		for j := range d.refs {
			if err := c.manager.DelRef(&d.refs[j]); err != nil {
				return err
			}
		}
		if len(values) == 1 {
			delete(c.macros, name)
		} else {
			c.macros[name] = values[:len(values)-1]
		}
		c.macroTrail = c.macroTrail[:i]
	}
	return nil
}

func (c *Context) compatibleSorts(a, b ast.ASTHandle) (bool, error) {
	if a == b {
		return true, nil
	}
	enabled, err := c.manager.IntRealCoercions()
	if err != nil || !enabled {
		return false, err
	}
	x, err := c.manager.GetFamilyID(a)
	if err != nil {
		return false, err
	}
	y, err := c.manager.GetFamilyID(b)
	if err != nil {
		return false, err
	}
	arith, err := c.manager.ArithmeticFamilyID()
	return x == arith && y == arith, err
}

// MakeAppInto mirrors the source expr_ref output contract. Caller owns out;
// substitution results stay alive after the temporary reducer is destroyed.
// MakeApp is the raw declaration/builtin construction helper.
func (c *Context) MakeAppInto(name string, args []ast.ASTHandle, indices []ast.ASTParameter, rangeSort ast.ASTHandle, out ast.ASTRef) (result error) {
	m, err := c.Manager()
	if err != nil {
		return err
	}
	if len(c.builtins[name]) == 0 && len(indices) == 0 {
		for _, d := range c.macros[name] {
			if len(d.domain) != len(args) {
				continue
			}
			matches := true
			for i, arg := range args {
				sort, err := m.GetSort(arg)
				if err != nil {
					return err
				}
				ok, err := c.compatibleSorts(d.domain[i], sort)
				if err != nil {
					return err
				}
				if !ok {
					matches = false
					break
				}
			}
			if !matches {
				continue
			}
			subst, err := rewriter.NewVarSubstitution(m, true)
			if subst != nil {
				defer func() { result = errors.Join(result, subst.Close()) }()
			}
			if err != nil {
				return err
			}
			if err = subst.Substitute(d.body, args, out); err != nil {
				return err
			}
			value, err := m.RefGet(out)
			if err != nil {
				return err
			}
			ok, err := m.IsWellSortedWithPrinter(value, nil, c.config.Params, &c.params, c.config.Globals)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("invalid macro application, sort mismatch %s", name)
			}
			return nil
		}
	}
	value, err := c.MakeApp(name, args, indices, rangeSort)
	if err != nil {
		return err
	}
	return m.SetRef(out, value)
}
