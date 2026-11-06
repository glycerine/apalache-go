package cmd_context

import "github.com/glycerine/g3/pkg/ast"

// BindFixedpointRule returns an owning formula with fixedpoint variables bound.
func (c *Context) BindFixedpointRule(formula ast.ASTHandle) (ast.ASTRef, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return ast.ASTRef{}, err
	}
	return engine.BindRule(formula)
}
