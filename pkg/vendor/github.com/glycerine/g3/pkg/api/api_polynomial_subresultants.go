package api

// Source: src/api/api_polynomial.cpp:25-81.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

func (c *SMTContext) PolynomialSubresultants(left, right, variable ast.ASTHandle) (result *APIPolynomialSubresultants, resultErr error) {
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	inputs, err := c.newPolynomialInputs(manager, left, right)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, inputs.close()) }()
	result = &APIPolynomialSubresultants{owner: c.smtContextState, manager: manager}
	ownedResult := result
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, ownedResult.Close())
			result = nil
		}
	}()
	isVariable, err := inputs.converter.IsVar(variable)
	if err != nil {
		return nil, err
	}
	if !isVariable {
		return result, nil
	}
	mapping, err := inputs.converter.Mapping()
	if err != nil {
		return nil, err
	}
	x, err := mapping.ToVar(variable)
	if err != nil {
		return nil, err
	}
	core, err := inputs.manager.ArithmeticCore()
	if err != nil {
		return nil, err
	}
	chain, err := polynomial.NewPolynomialRefVector(core)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, chain.Close()) }()
	interrupt, err := newAPIPolynomialInterruptScope(c)
	if err != nil {
		return nil, err
	}
	err = inputs.manager.PSCChain(inputs.leftValue, inputs.rightValue, x, chain)
	err = errors.Join(err, interrupt.Close())
	if err != nil {
		return nil, err
	}
	size, err := chain.Size()
	if err != nil {
		return nil, err
	}
	result.values = make([]ast.ASTRef, 0, size)
	for i := uint32(0); i < size; i++ {
		value, e := chain.Get(i)
		if e != nil {
			return nil, e
		}
		ref, e := inputs.converter.ToExpr(value, true)
		if e != nil {
			return nil, e
		}
		result.values = append(result.values, ref)
	}
	return result, nil
}
