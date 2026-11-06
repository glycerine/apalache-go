package ast

// Source: src/api/api_pb.cpp integer coefficient conversion at the public API
// boundary. The underlying PB utility accepts rationals, but all five public
// constructors supply uint32 or int32 values.

import "math/big"

func (m *ASTManager) pseudoBooleanIntegerRelation(kind int32, coefficients []int32, args []ASTHandle, bound int32) (ASTHandle, error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, err
	}
	defer m.mu.Unlock()
	if len(coefficients) != len(args) {
		return ASTHandle{}, ErrASTDomain
	}
	values := make([]*big.Int, len(coefficients))
	for index, coefficient := range coefficients {
		values[index] = big.NewInt(int64(coefficient))
	}
	return m.pseudoBooleanBuild(kind, values, big.NewInt(int64(bound)), args)
}

// MkPseudoBooleanLEIntegers constructs the API's integer-coefficient <= form.
func (m *ASTManager) MkPseudoBooleanLEIntegers(coefficients []int32, args []ASTHandle, bound int32) (ASTHandle, error) {
	return m.pseudoBooleanIntegerRelation(ASTPBOpLE, coefficients, args, bound)
}

// MkPseudoBooleanGEIntegers constructs the API's integer-coefficient >= form.
func (m *ASTManager) MkPseudoBooleanGEIntegers(coefficients []int32, args []ASTHandle, bound int32) (ASTHandle, error) {
	return m.pseudoBooleanIntegerRelation(ASTPBOpGE, coefficients, args, bound)
}

// MkPseudoBooleanEQIntegers constructs the API's integer-coefficient equality.
func (m *ASTManager) MkPseudoBooleanEQIntegers(coefficients []int32, args []ASTHandle, bound int32) (ASTHandle, error) {
	return m.pseudoBooleanIntegerRelation(ASTPBOpEQ, coefficients, args, bound)
}
