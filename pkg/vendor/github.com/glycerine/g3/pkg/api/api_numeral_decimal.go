package api

// Source: src/api/api_numeral.cpp:231-263.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

// NumeralDecimalString renders arithmetic rational and irrational algebraic
// numerals using the source decimal algorithms. Integer, bit-vector and
// finite-domain numerals keep their exact rational spelling.
func (c *SMTContext) NumeralDecimalString(value ast.ASTHandle, precision uint32) (string, error) {
	manager, err := c.Manager()
	if err != nil {
		return "", err
	}
	if text, _, matched, readErr := manager.ArithmeticNumeralText(value); readErr != nil {
		return "", readErr
	} else if matched {
		return apiRationalDecimalString(manager, text, precision)
	}
	var number polynomial.Algebraic
	matched, err := manager.ArithmeticAlgebraicNumeralValue(value, &number)
	if err != nil {
		return "", err
	}
	if matched {
		return apiAlgebraicDecimalString(manager, &number, precision)
	}
	if rational, matched, err := c.NumeralRational(value); err != nil {
		return "", err
	} else if matched {
		return rational.RatString(), nil
	}
	return "", errAPINumeralArgument
}
