package api

// Source: src/api/api_numeral.cpp:241-246; src/util/mpq.cpp display_decimal.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func apiRationalDecimalString(manager *ast.ASTManager, text string, precision uint32) (result string, resultErr error) {
	rational, ok := new(big.Rat).SetString(text)
	if !ok {
		return "", ast.ErrASTDomain
	}
	if rational.IsInt() {
		return rational.RatString(), nil
	}
	service := manager.NumeralService()
	if service == nil {
		return "", ast.ErrASTHandle
	}
	value, err := service.NewValue()
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, service.Del(&value)) }()
	if err = service.SetString(&value, text); err != nil {
		return "", err
	}
	return service.DecimalString(&value, precision, false)
}
