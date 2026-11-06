package api

// Source: src/api/api_quant.cpp:281-289.

import "github.com/glycerine/g3/pkg/ast"

// BoundVariable constructs a de Bruijn variable with the supplied index and
// sort. The returned handle is borrowed from the context's AST manager.
func (c *SMTContext) BoundVariable(index uint32, sort ast.ASTHandle) (ast.ASTHandle, error) {
	manager, err := c.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkVar(index, sort)
}
