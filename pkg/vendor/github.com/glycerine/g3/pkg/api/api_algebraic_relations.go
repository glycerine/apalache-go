package api

// Source: src/api/api_algebraic.cpp:256-324.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) compareAlgebraic(left, right ast.ASTHandle) (result int, resultErr error) {
	manager, err := c.Manager()
	if err != nil {
		return 0, err
	}
	values, err := readAPIAlgebraicPair(manager, left, right)
	if err != nil {
		return 0, err
	}
	defer func() {
		if closeErr := values.close(); resultErr == nil {
			resultErr = closeErr
		}
	}()
	return values.left.manager.Compare(&values.left.value, &values.right.value)
}

func (c *SMTContext) AlgebraicLT(left, right ast.ASTHandle) (bool, error) {
	order, err := c.compareAlgebraic(left, right)
	return order < 0 && err == nil, err
}

func (c *SMTContext) AlgebraicGT(left, right ast.ASTHandle) (bool, error) {
	order, err := c.compareAlgebraic(left, right)
	return order > 0 && err == nil, err
}

func (c *SMTContext) AlgebraicLE(left, right ast.ASTHandle) (bool, error) {
	order, err := c.compareAlgebraic(left, right)
	return order <= 0 && err == nil, err
}

func (c *SMTContext) AlgebraicGE(left, right ast.ASTHandle) (bool, error) {
	order, err := c.compareAlgebraic(left, right)
	return order >= 0 && err == nil, err
}

func (c *SMTContext) AlgebraicEqual(left, right ast.ASTHandle) (bool, error) {
	order, err := c.compareAlgebraic(left, right)
	return order == 0 && err == nil, err
}

func (c *SMTContext) AlgebraicNotEqual(left, right ast.ASTHandle) (bool, error) {
	order, err := c.compareAlgebraic(left, right)
	return order != 0 && err == nil, err
}
