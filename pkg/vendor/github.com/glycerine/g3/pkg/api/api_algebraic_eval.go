package api

// Source: src/api/api_algebraic.cpp:389-415.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// AlgebraicEval returns the sign of expression after assigning variables
// 0..len(assignments)-1 to the supplied algebraic values.
func (c *SMTContext) AlgebraicEval(expression ast.ASTHandle, assignments []ast.ASTHandle) (result int, resultErr error) {
	manager, err := c.Manager()
	if err != nil {
		return 0, err
	}
	if uint64(len(assignments)) > math.MaxUint32 {
		return 0, arena.ErrBounds
	}
	p, err := c.newAlgebraicPolynomial(manager, expression)
	if err != nil {
		return 0, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.close()) }()
	maxVariable, err := p.maxVariable()
	if err != nil {
		return 0, err
	}
	if maxVariable >= uint32(len(assignments)) {
		return 0, ast.ErrASTDomain
	}
	values, err := newAPIAlgebraicAssignments(manager, assignments)
	if err != nil {
		return 0, err
	}
	defer func() { resultErr = errors.Join(resultErr, values.close()) }()
	algebraic, err := manager.ArithmeticAlgebraicManager()
	if err != nil {
		return 0, err
	}
	evaluationManager, evaluationValue, evaluationRef, err := p.copyForAlgebraic(algebraic)
	if err != nil {
		return 0, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluationRef.Close()) }()

	err = func() (operationErr error) {
		interrupt, operationErr := newAPIOperationInterruptScope(c)
		if operationErr != nil {
			return operationErr
		}
		defer func() { operationErr = errors.Join(operationErr, interrupt.Close()) }()
		result, operationErr = algebraic.EvalSign(evaluationManager, evaluationValue, &values)
		return operationErr
	}()
	if result > 0 {
		result = 1
	} else if result < 0 {
		result = -1
	}
	return result, err
}
