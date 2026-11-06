package api

// Source: src/api/api_arith.cpp:126-166.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// IsIrrationalAlgebraicNumber follows Z3_is_algebraic_number. Rational
// numerals are algebraic values mathematically, but this source predicate is
// deliberately true only for the manager's irrational algebraic numerals.
func (c *SMTContext) IsIrrationalAlgebraicNumber(value ast.ASTHandle) (result bool, resultErr error) {
	manager, err := c.Manager()
	if err != nil {
		return false, err
	}
	var number polynomial.Algebraic
	matched, err := manager.ArithmeticAlgebraicNumeralValue(value, &number)
	if err != nil || !matched {
		return false, err
	}
	algebraic, err := manager.ArithmeticAlgebraicManager()
	if err != nil {
		return false, err
	}
	return true, algebraic.Del(&number)
}

func (c *SMTContext) algebraicBound(value ast.ASTHandle, precision uint32, upper bool) (result ast.ASTHandle, resultErr error) {
	manager, err := c.Manager()
	if err != nil {
		return result, err
	}
	var number polynomial.Algebraic
	matched, err := manager.ArithmeticAlgebraicNumeralValue(value, &number)
	if err != nil {
		return result, err
	}
	if !matched {
		return result, ast.ErrASTDomain
	}
	algebraic, err := manager.ArithmeticAlgebraicManager()
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, algebraic.Del(&number)) }()
	rationals, err := algebraic.RationalManager()
	if err != nil {
		return result, err
	}
	bound := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, rationals.Del(&bound)) }()
	if upper {
		err = algebraic.GetUpperRationalAtPrecision(&number, &bound, precision)
	} else {
		err = algebraic.GetLowerRationalAtPrecision(&number, &bound, precision)
	}
	if err != nil {
		return result, err
	}
	text, err := rationals.String(&bound)
	if err != nil {
		return result, err
	}
	return manager.ArithmeticRealNumeralText(text)
}

// AlgebraicNumberLower returns the rational lower approximation selected at
// the source precision.
func (c *SMTContext) AlgebraicNumberLower(value ast.ASTHandle, precision uint32) (ast.ASTHandle, error) {
	return c.algebraicBound(value, precision, false)
}

// AlgebraicNumberUpper returns the rational upper approximation selected at
// the source precision.
func (c *SMTContext) AlgebraicNumberUpper(value ast.ASTHandle, precision uint32) (ast.ASTHandle, error) {
	return c.algebraicBound(value, precision, true)
}
