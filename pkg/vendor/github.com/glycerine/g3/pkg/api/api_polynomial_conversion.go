package api

// Source: src/api/api_polynomial.cpp:34-61 shared converter and polynomial
// owners used by Z3_polynomial_subresultants.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type apiPolynomialInputs struct {
	converter                         *ast.Expr2Polynomial
	left, right                       *polynomial.PolynomialRef
	leftDenominator, rightDenominator *unumeral.ScopedInt
	manager                           *polynomial.MultivariatePolynomialManager
	leftValue, rightValue             polynomial.Polynomial
}

func (p *apiPolynomialInputs) close() (result error) {
	if p == nil {
		return nil
	}
	if p.converter != nil {
		result = errors.Join(result, p.converter.Close())
		p.converter = nil
	}
	if p.rightDenominator != nil {
		result = errors.Join(result, p.rightDenominator.Close())
		p.rightDenominator = nil
	}
	if p.leftDenominator != nil {
		result = errors.Join(result, p.leftDenominator.Close())
		p.leftDenominator = nil
	}
	if p.right != nil {
		result = errors.Join(result, p.right.Close())
		p.right = nil
	}
	if p.left != nil {
		result = errors.Join(result, p.left.Close())
		p.left = nil
	}
	p.manager = nil
	return result
}

func (c *SMTContext) newPolynomialInputs(manager *ast.ASTManager, left, right ast.ASTHandle) (result apiPolynomialInputs, resultErr error) {
	var err error
	result.manager, err = c.polynomialManager()
	if err != nil {
		return result, err
	}
	result.converter, err = ast.NewExpr2Polynomial(ast.Expr2PolynomialConfig{AST: manager, Polynomial: result.manager})
	if err != nil {
		return result, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.close())
		}
	}()
	var matched bool
	result.left, result.leftDenominator, matched, err = result.converter.ToPolynomial(left)
	if err != nil {
		return result, err
	}
	if !matched {
		return result, ast.ErrASTDomain
	}
	result.right, result.rightDenominator, matched, err = result.converter.ToPolynomial(right)
	if err != nil {
		return result, err
	}
	if !matched {
		return result, ast.ErrASTDomain
	}
	result.leftValue, err = result.left.Get()
	if err != nil {
		return result, err
	}
	result.rightValue, err = result.right.Get()
	if err != nil {
		return result, err
	}
	return result, nil
}
