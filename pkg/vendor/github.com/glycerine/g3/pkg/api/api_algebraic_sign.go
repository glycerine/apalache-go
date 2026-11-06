package api

// Source: src/api/api_algebraic.cpp:82-125.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) AlgebraicSign(h ast.ASTHandle) (result int, resultErr error) {
	manager, err := c.Manager()
	if err != nil {
		return 0, err
	}
	value, err := readAPIAlgebraicInput(manager, h)
	if err != nil {
		return 0, err
	}
	defer func() {
		if closeErr := value.close(); resultErr == nil {
			resultErr = closeErr
		}
	}()
	positive, err := value.manager.IsPos(&value.value)
	if err != nil {
		return 0, err
	}
	if positive {
		return 1, nil
	}
	negative, err := value.manager.IsNeg(&value.value)
	if err != nil {
		return 0, err
	}
	if negative {
		return -1, nil
	}
	return 0, nil
}

func (c *SMTContext) AlgebraicIsPositive(h ast.ASTHandle) (bool, error) {
	sign, err := c.AlgebraicSign(h)
	return sign > 0 && err == nil, err
}

func (c *SMTContext) AlgebraicIsNegative(h ast.ASTHandle) (bool, error) {
	sign, err := c.AlgebraicSign(h)
	return sign < 0 && err == nil, err
}

func (c *SMTContext) AlgebraicIsZero(h ast.ASTHandle) (bool, error) {
	sign, err := c.AlgebraicSign(h)
	return sign == 0 && err == nil, err
}
