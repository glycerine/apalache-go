package cmd_context

// Source: src/cmd_context/pdecl.cpp/.h user sort declarations, applications,
// caches and source sort substitution; src/cmd_context/cmd_context.cpp trails.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type sortConstructorDeclaration struct {
	name       string
	arity      uint32
	symbol     usymbol.Symbol
	formals    []ast.ASTHandle
	body       ast.ASTHandle
	definition bool
	refs       []ast.ASTRef
}

func (c *Context) sortNameAvailable(name string) bool {
	_, sortFound := c.sorts[name]
	_, constructorFound := c.sortConstructors[name]
	return sortFound || constructorFound || c.BuiltinSortAvailable(name)
}

// DeclareSortConstructor installs an uninterpreted sort constructor. Arity
// zero retains the existing concrete DeclareSort behavior.
func (c *Context) DeclareSortConstructor(name string, arity uint32) (ast.ASTHandle, error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if c.sortNameAvailable(name) {
		return ast.ASTHandle{}, fmt.Errorf("sort already defined %s", name)
	}
	symbol, err := m.Symbols.Intern(name)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if arity == 0 {
		sort, e := m.MkUninterpretedSort(symbol, nil)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		ref, e := m.NewRef(sort)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		c.sorts[name] = sort
		c.sortTrail = append(c.sortTrail, declaration{name, sort, ref})
		return sort, nil
	}
	declaration := sortConstructorDeclaration{name: name, arity: arity, symbol: symbol}
	c.sortConstructors[name] = declaration
	c.sortConstructorTrail = append(c.sortConstructorTrail, name)
	return ast.ASTHandle{}, nil
}

// DefineSort installs a concrete sort alias or a parametric sort definition.
// The context acquires the body and formal roots before parser-local roots end.
func (c *Context) DefineSort(name string, formals []ast.ASTHandle, body ast.ASTHandle) (err error) {
	m, err := c.Manager()
	if err != nil {
		return err
	}
	if c.sortNameAvailable(name) {
		return fmt.Errorf("sort already defined %s", name)
	}
	if len(formals) == 0 {
		ref, e := m.NewRef(body)
		if e != nil {
			return e
		}
		c.sorts[name] = body
		c.sortTrail = append(c.sortTrail, declaration{name, body, ref})
		return nil
	}
	symbol, err := m.Symbols.Intern(name)
	if err != nil {
		return err
	}
	declaration := sortConstructorDeclaration{name: name, arity: uint32(len(formals)), symbol: symbol, formals: append([]ast.ASTHandle(nil), formals...), body: body, definition: true}
	declaration.refs = make([]ast.ASTRef, 0, len(formals)+1)
	defer func() {
		if err == nil {
			return
		}
		for i := len(declaration.refs) - 1; i >= 0; i-- {
			err = errors.Join(err, m.DelRef(&declaration.refs[i]))
		}
	}()
	for _, formal := range formals {
		ref, e := m.NewRef(formal)
		if e != nil {
			return e
		}
		declaration.refs = append(declaration.refs, ref)
	}
	ref, err := m.NewRef(body)
	if err != nil {
		return err
	}
	declaration.refs = append(declaration.refs, ref)
	c.sortConstructors[name] = declaration
	c.sortConstructorTrail = append(c.sortConstructorTrail, name)
	return nil
}

func (c *Context) instantiateSortConstructor(declaration sortConstructorDeclaration, arguments []ast.ASTHandle) (result ast.ASTHandle, err error) {
	if uint32(len(arguments)) != declaration.arity {
		return ast.ASTHandle{}, fmt.Errorf("invalid number of parameters to sort constructor %s: got %d, want %d", declaration.name, len(arguments), declaration.arity)
	}
	if declaration.definition {
		return c.manager.SubstituteSort(declaration.body, declaration.formals, arguments)
	}
	parameters := make([]ast.ASTParameter, 0, len(arguments))
	created := make([]ast.ASTParameter, 0, len(arguments))
	defer func() {
		for i := len(created) - 1; i >= 0; i-- {
			err = errors.Join(err, c.manager.DelParameter(&created[i]))
		}
	}()
	for _, argument := range arguments {
		parameter, e := c.manager.NewParameterAST(argument)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		created = append(created, parameter)
		parameters = append(parameters, parameter)
	}
	return c.manager.MkUninterpretedSort(declaration.symbol, parameters)
}

func (c *Context) restoreSortConstructors(size int) (result error) {
	for len(c.sortConstructorTrail) > size {
		i := len(c.sortConstructorTrail) - 1
		name := c.sortConstructorTrail[i]
		declaration := c.sortConstructors[name]
		delete(c.sortConstructors, name)
		for j := len(declaration.refs) - 1; j >= 0; j-- {
			result = errors.Join(result, c.manager.DelRef(&declaration.refs[j]))
		}
		c.sortConstructorTrail = c.sortConstructorTrail[:i]
	}
	return result
}
