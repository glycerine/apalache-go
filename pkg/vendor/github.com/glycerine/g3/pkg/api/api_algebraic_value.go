package api

// Source: src/api/api_algebraic.cpp:35-79,326-351.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type apiAlgebraicInput struct {
	manager *polynomial.AlgebraicManager
	value   polynomial.Algebraic
}

func (v *apiAlgebraicInput) close() error {
	if v == nil || v.manager == nil {
		return nil
	}
	err := v.manager.Del(&v.value)
	v.manager = nil
	return err
}

type apiAlgebraicPair struct {
	left, right apiAlgebraicInput
}

func (p *apiAlgebraicPair) close() error {
	return errors.Join(p.right.close(), p.left.close())
}

func readAPIAlgebraicInput(manager *ast.ASTManager, h ast.ASTHandle) (result apiAlgebraicInput, resultErr error) {
	algebraic, err := manager.ArithmeticAlgebraicManager()
	if err != nil {
		return result, err
	}
	result.manager = algebraic
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.close())
		}
	}()
	matched, err := manager.ArithmeticAlgebraicNumeralValue(h, &result.value)
	if err != nil {
		return result, err
	}
	if matched {
		return result, nil
	}
	text, _, matched, err := manager.ArithmeticNumeralText(h)
	if err != nil {
		return result, err
	}
	if !matched {
		return result, ast.ErrASTDomain
	}
	rationals, err := algebraic.RationalManager()
	if err != nil {
		return result, err
	}
	var q unumeral.Rational
	defer func() { resultErr = errors.Join(resultErr, rationals.Del(&q)) }()
	if err = rationals.SetString(&q, text); err != nil {
		return result, err
	}
	if err = algebraic.SetRational(&result.value, &q); err != nil {
		return result, err
	}
	return result, nil
}

func readAPIAlgebraicPair(manager *ast.ASTManager, left, right ast.ASTHandle) (result apiAlgebraicPair, resultErr error) {
	result.left, resultErr = readAPIAlgebraicInput(manager, left)
	if resultErr != nil {
		return result, resultErr
	}
	result.right, resultErr = readAPIAlgebraicInput(manager, right)
	if resultErr != nil {
		return result, errors.Join(resultErr, result.left.close())
	}
	return result, nil
}

func (c *SMTContext) IsAlgebraicValue(h ast.ASTHandle) (bool, error) {
	manager, err := c.Manager()
	if err != nil {
		return false, err
	}
	var value polynomial.Algebraic
	matched, err := manager.ArithmeticAlgebraicNumeralValue(h, &value)
	if err != nil {
		return false, err
	}
	if matched {
		algebraic, err := manager.ArithmeticAlgebraicManager()
		if err != nil {
			return false, err
		}
		return true, algebraic.Del(&value)
	}
	_, _, matched, err = manager.ArithmeticNumeralText(h)
	return matched, err
}
