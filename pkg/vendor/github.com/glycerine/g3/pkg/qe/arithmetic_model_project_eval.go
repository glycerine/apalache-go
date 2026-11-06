package qe

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *arithmeticModelProjectCall) evaluate(expression ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.evaluator.Evaluate(expression, c.value); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.plugin.manager.RefGet(c.value)
}

func (c *arithmeticModelProjectCall) truth(expression ast.ASTHandle) (bool, error) {
	value, err := c.evaluate(expression)
	return err == nil && c.plugin.manager.IsTrue(value), err
}

func (c *arithmeticModelProjectCall) numeral(expression ast.ASTHandle) (*big.Rat, error) {
	value, err := c.evaluate(expression)
	if err != nil {
		return nil, err
	}
	number, matched, err := c.plugin.util.numeral(value)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, ast.ErrASTDomain
	}
	return number, nil
}
