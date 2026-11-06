package base

// Source: src/muz/base/dl_context.cpp context::get_background_assertion.

import "github.com/glycerine/g3/pkg/ast"

func (c *DatalogContext) BackgroundFormula() (ast.ASTRef, error) {
	if err := c.open(); err != nil {
		return ast.ASTRef{}, err
	}
	switch len(c.background) {
	case 0:
		truth, err := c.manager.MkTrue()
		if err != nil {
			return ast.ASTRef{}, err
		}
		return c.manager.NewRef(truth)
	case 1:
		return c.manager.CopyRef(c.background[0])
	}
	arguments := make([]ast.ASTHandle, len(c.background))
	for i, reference := range c.background {
		handle, err := c.manager.RefGet(reference)
		if err != nil {
			return ast.ASTRef{}, err
		}
		arguments[i] = handle
	}
	conjunction, err := c.manager.MkAndArgs(arguments)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return c.manager.NewRef(conjunction)
}
