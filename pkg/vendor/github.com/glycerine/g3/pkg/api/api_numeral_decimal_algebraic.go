package api

// Source: src/api/api_numeral.cpp:247-254.

import (
	"bytes"
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

func apiAlgebraicDecimalString(manager *ast.ASTManager, value *polynomial.Algebraic, precision uint32) (result string, resultErr error) {
	algebraic, err := manager.ArithmeticAlgebraicManager()
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, algebraic.Del(value)) }()
	var output bytes.Buffer
	if err = algebraic.DisplayDecimal(&output, value, precision); err != nil {
		return "", err
	}
	return output.String(), nil
}
