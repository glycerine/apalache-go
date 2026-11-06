package api

// Source: src/api/api_algebraic.cpp BIN_OP and algebraic root/power result conversion.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

func makeAPIAlgebraicResult(manager *ast.ASTManager, algebraic *polynomial.AlgebraicManager, value *polynomial.Algebraic) (result ast.ASTHandle, resultErr error) {
	rational, err := algebraic.IsRational(value)
	if err != nil {
		return result, err
	}
	if !rational {
		return manager.ArithmeticAlgebraicNumeral(value)
	}
	service := manager.NumeralService()
	if service == nil {
		return result, ast.ErrASTHandle
	}
	q, err := service.NewValue()
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, service.CleanupValue(&q)) }()
	if err = algebraic.ToOwnedRational(value, service, &q); err != nil {
		return result, err
	}
	return manager.ArithmeticNumeral(&q, false)
}
