package api

// Source: src/api/api_algebraic.cpp:127-204.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

type apiAlgebraicBinaryOperation uint8

const (
	apiAlgebraicAdd apiAlgebraicBinaryOperation = iota
	apiAlgebraicSub
	apiAlgebraicMul
	apiAlgebraicDiv
)

func (c *SMTContext) algebraicBinary(left, right ast.ASTHandle, operation apiAlgebraicBinaryOperation) (result ast.ASTHandle, resultErr error) {
	manager, err := c.Manager()
	if err != nil {
		return result, err
	}
	values, err := readAPIAlgebraicPair(manager, left, right)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, values.close()) }()
	algebraic := values.left.manager
	var output polynomial.Algebraic
	defer func() { resultErr = errors.Join(resultErr, algebraic.Del(&output)) }()
	switch operation {
	case apiAlgebraicAdd:
		err = algebraic.Add(&values.left.value, &values.right.value, &output)
	case apiAlgebraicSub:
		err = algebraic.Sub(&values.left.value, &values.right.value, &output)
	case apiAlgebraicMul:
		err = algebraic.Mul(&values.left.value, &values.right.value, &output)
	case apiAlgebraicDiv:
		zero, zeroErr := algebraic.IsZero(&values.right.value)
		if zeroErr != nil {
			return result, zeroErr
		}
		if zero {
			return result, ast.ErrASTDomain
		}
		err = algebraic.Div(&values.left.value, &values.right.value, &output)
	default:
		return result, ast.ErrASTDomain
	}
	if err != nil {
		return result, err
	}
	return makeAPIAlgebraicResult(manager, algebraic, &output)
}

func (c *SMTContext) AlgebraicAdd(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	return c.algebraicBinary(left, right, apiAlgebraicAdd)
}

func (c *SMTContext) AlgebraicSub(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	return c.algebraicBinary(left, right, apiAlgebraicSub)
}

func (c *SMTContext) AlgebraicMul(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	return c.algebraicBinary(left, right, apiAlgebraicMul)
}

func (c *SMTContext) AlgebraicDiv(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	return c.algebraicBinary(left, right, apiAlgebraicDiv)
}
