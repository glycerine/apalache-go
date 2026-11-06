package cmd_context

// Source: src/cmd_context/basic_cmds.cpp declare_map_cmd.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

// DeclareMap publishes a named array-map declaration over the supplied index
// sorts and mapped function declaration.
func (c *Context) DeclareMap(name string, indices []ast.ASTHandle, function ast.ASTHandle) (result ast.ASTHandle, err error) {
	if len(indices) == 0 {
		return result, fmt.Errorf("invalid map declaration, empty sort list")
	}
	m, err := c.Manager()
	if err != nil {
		return result, err
	}
	arity, err := m.GetArity(function)
	if err != nil {
		return result, err
	}
	if arity == 0 {
		return result, fmt.Errorf("invalid map declaration, function declaration must have arity > 0")
	}
	domain := make([]ast.ASTHandle, arity)
	for i := uint32(0); i < arity; i++ {
		valueSort, e := m.GetDomain(function, i)
		if e != nil {
			return result, e
		}
		domain[i], e = m.MkArraySort(ast.SliceArguments(indices), valueSort)
		if e != nil {
			return result, e
		}
	}
	valueRange, err := m.GetRange(function)
	if err != nil {
		return result, err
	}
	rangeSort, err := m.MkArraySort(ast.SliceArguments(indices), valueRange)
	if err != nil {
		return result, err
	}
	parameter, err := m.NewParameterAST(function)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, m.DelParameter(&parameter)) }()
	family, err := m.RegisterArrayPlugin()
	if err != nil {
		return result, err
	}
	declaration, err := m.MkFamilyFuncDecl(family, ast.ASTArrayOpMap, []ast.ASTParameter{parameter}, domain, rangeSort)
	if err != nil {
		return result, fmt.Errorf("invalid array map operator: %w", err)
	}
	return c.publishFunctionDeclaration(name, declaration)
}
