package qe

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *ArithmeticPlugin) equationRest(equation *arithmeticEquation, variables []ast.ASTHandle, excluded int, template ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	var owned []ast.ASTRef
	defer func() {
		for index := len(owned) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&owned[index]))
		}
	}()
	terms := make([]ast.ASTHandle, 0, len(variables)+len(equation.opaque)+1)
	if equation.constant.Sign() != 0 {
		constant, err := p.util.numeralFor(equation.constant, template)
		if err != nil {
			return result, err
		}
		terms = append(terms, constant)
	}
	for index, coefficient := range equation.coefficients {
		if index == excluded || coefficient.Sign() == 0 {
			continue
		}
		term, err := p.util.scale(coefficient, variables[index])
		if err != nil {
			return result, err
		}
		owned = append(owned, term)
		handle, err := p.manager.RefGet(term)
		if err != nil {
			return result, err
		}
		terms = append(terms, handle)
	}
	for _, opaque := range equation.opaque {
		term, err := p.util.scale(opaque.coefficient, opaque.expression)
		if err != nil {
			return result, err
		}
		owned = append(owned, term)
		handle, err := p.manager.RefGet(term)
		if err != nil {
			return result, err
		}
		terms = append(terms, handle)
	}
	if len(terms) == 0 {
		zero, err := p.util.zero(template)
		if err != nil {
			return result, err
		}
		return p.manager.NewRef(zero)
	}
	return p.util.sum(terms, template)
}

func arithmeticEquationInteger(value *big.Rat) (*big.Int, bool) {
	if value == nil || !value.IsInt() {
		return nil, false
	}
	return new(big.Int).Set(value.Num()), true
}
