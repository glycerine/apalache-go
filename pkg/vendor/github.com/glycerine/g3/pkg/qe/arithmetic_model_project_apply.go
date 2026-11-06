package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *arithmeticModelProjectCall) apply(variable *ast.ContainsApp, formula ast.ASTRef, branch uint32) (resultErr error) {
	service := c.plugin.manager.NumeralService()
	value, err := service.NewValue()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, service.Del(&value)) }()
	if err = service.SetUnsigned(&value, branch); err != nil {
		return err
	}
	root, err := c.plugin.manager.RefGet(formula)
	if err != nil {
		return err
	}
	if err = c.plugin.Assign(variable, root, &value); err != nil {
		return err
	}
	return c.plugin.Substitute(variable, &value, formula, nil)
}
