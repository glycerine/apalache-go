package api

// Source: src/api/api_algebraic.cpp:206-254.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

func (c *SMTContext) algebraicUnaryPower(h ast.ASTHandle, exponent uint32, root bool) (result ast.ASTHandle, resultErr error) {
	manager, err := c.Manager()
	if err != nil {
		return result, err
	}
	input, err := readAPIAlgebraicInput(manager, h)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, input.close()) }()
	if root && exponent%2 == 0 {
		negative, e := input.manager.IsNeg(&input.value)
		if e != nil {
			return result, e
		}
		if negative {
			return result, ast.ErrASTDomain
		}
	}
	var output polynomial.Algebraic
	defer func() { resultErr = errors.Join(resultErr, input.manager.Del(&output)) }()
	if root {
		err = input.manager.Root(&input.value, exponent, &output)
	} else {
		err = input.manager.Power(&input.value, exponent, &output)
	}
	if err != nil {
		return result, err
	}
	return makeAPIAlgebraicResult(manager, input.manager, &output)
}

func (c *SMTContext) AlgebraicRoot(h ast.ASTHandle, degree uint32) (ast.ASTHandle, error) {
	return c.algebraicUnaryPower(h, degree, true)
}

func (c *SMTContext) AlgebraicPower(h ast.ASTHandle, exponent uint32) (ast.ASTHandle, error) {
	return c.algebraicUnaryPower(h, exponent, false)
}
