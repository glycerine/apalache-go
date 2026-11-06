package base

import "github.com/glycerine/g3/pkg/ast"

func (c *DatalogContext) RegisterVariable(declaration ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	variable, err := c.manager.MkApp(declaration, nil)
	if err != nil {
		return err
	}
	return c.binder.AddVariable(variable)
}

func (c *DatalogContext) SetBindVariables(enabled bool) error {
	if err := c.open(); err != nil {
		return err
	}
	c.bindVariables = enabled
	return nil
}

func (c *DatalogContext) BindVariables(expression ast.ASTHandle, forall bool) (ast.ASTRef, error) {
	if err := c.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if !c.bindVariables {
		return c.manager.NewRef(expression)
	}
	return c.binder.Bind(expression, forall)
}
