package cmd_context

// Source: src/api/api_parsers.cpp:51-63 supplied declaration installation.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

// InsertFunction installs an existing same-manager declaration under the
// parser-visible name and acquires the context's normal declaration reference.
func (c *Context) InsertFunction(name string, declaration ast.ASTHandle) (ast.ASTHandle, error) {
	manager, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	kind, err := manager.GetKind(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if kind != ast.ASTFuncDecl {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return c.publishFunctionDeclaration(name, declaration)
}

// InsertSort installs an existing same-manager sort as a zero-arity parser
// sort declaration.
func (c *Context) InsertSort(name string, sort ast.ASTHandle) (err error) {
	manager, err := c.Manager()
	if err != nil {
		return err
	}
	kind, err := manager.GetKind(sort)
	if err != nil {
		return err
	}
	if kind != ast.ASTSort {
		return ast.ErrASTDomain
	}
	if c.sortNameAvailable(name) {
		return fmt.Errorf("sort already defined %s", name)
	}
	ref, err := manager.NewRef(sort)
	if err != nil {
		return err
	}
	c.sorts[name] = sort
	c.sortTrail = append(c.sortTrail, declaration{name: name, handle: sort, ref: ref})
	return nil
}
