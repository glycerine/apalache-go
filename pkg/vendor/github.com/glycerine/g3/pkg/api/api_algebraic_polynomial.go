package api

// Source: src/api/api_algebraic.cpp:326-415 expression-to-polynomial setup.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type apiAlgebraicPolynomial struct {
	converter   *ast.Expr2Polynomial
	polynomial  *polynomial.PolynomialRef
	denominator *unumeral.ScopedInt
	manager     *polynomial.MultivariatePolynomialManager
	value       polynomial.Polynomial
}

func (p *apiAlgebraicPolynomial) close() (result error) {
	if p == nil {
		return nil
	}
	if p.converter != nil {
		result = errors.Join(result, p.converter.Close())
		p.converter = nil
	}
	if p.denominator != nil {
		result = errors.Join(result, p.denominator.Close())
		p.denominator = nil
	}
	if p.polynomial != nil {
		result = errors.Join(result, p.polynomial.Close())
		p.polynomial = nil
	}
	p.manager = nil
	p.value = polynomial.Polynomial{}
	return result
}

func (c *SMTContext) newAlgebraicPolynomial(manager *ast.ASTManager, expression ast.ASTHandle) (result apiAlgebraicPolynomial, resultErr error) {
	var err error
	result.manager, err = c.polynomialManager()
	if err != nil {
		return result, err
	}
	result.converter, err = ast.NewExpr2Polynomial(ast.Expr2PolynomialConfig{AST: manager, Polynomial: result.manager, UseVarIndices: true})
	if err != nil {
		return result, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.close())
		}
	}()
	var matched bool
	result.polynomial, result.denominator, matched, err = result.converter.ToPolynomial(expression)
	if err != nil {
		return result, err
	}
	if !matched {
		return result, ast.ErrASTDomain
	}
	result.value, err = result.polynomial.Get()
	if err != nil {
		return result, err
	}
	return result, nil
}

func (p *apiAlgebraicPolynomial) maxVariable() (uint32, error) {
	core, err := p.manager.ArithmeticCore()
	if err != nil {
		return 0, err
	}
	return core.MaxVariable(p.value)
}
