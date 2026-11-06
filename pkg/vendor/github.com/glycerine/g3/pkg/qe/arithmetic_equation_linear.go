package qe

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *ArithmeticPlugin) solveLinearIntegerEquation(equation *arithmeticEquation, variables []ast.ASTHandle, formula ast.ASTHandle) (*ArithmeticEquationElimination, bool, error) {
	if len(equation.opaque) != 0 || len(variables) == 0 {
		return nil, false, nil
	}
	row := make([]*big.Int, len(variables)+1)
	constant, integer := arithmeticEquationInteger(equation.constant)
	if !integer {
		return nil, false, nil
	}
	row[0] = constant
	for index, variable := range variables {
		coefficient, ok := arithmeticEquationInteger(equation.coefficients[index])
		if !ok {
			return nil, false, nil
		}
		if coefficient.Sign() != 0 {
			isInteger, cause := p.util.isInteger(variable)
			if cause != nil {
				return nil, false, cause
			}
			if !isInteger {
				return nil, false, nil
			}
		}
		row[index+1] = coefficient
	}
	rowIndex, fresh, satisfiable, err := solveArithmeticIntegerRow(row)
	if err != nil || !satisfiable {
		return nil, false, err
	}
	index := rowIndex - 1
	if index < 0 || index >= len(variables) || row[rowIndex].Sign() == 0 {
		return nil, false, nil
	}
	transformed := &arithmeticEquation{constant: new(big.Rat).SetInt(row[0]), coefficients: make([]*big.Rat, len(variables))}
	for position := range variables {
		transformed.coefficients[position] = new(big.Rat).SetInt(row[position+1])
	}
	transformed.coefficients[index].SetInt64(0)
	var freshRefs []ast.ASTRef
	if fresh {
		sort, cause := p.manager.GetSort(variables[index])
		if cause != nil {
			return nil, false, cause
		}
		auxiliary, cause := p.manager.MkFreshConst("qe-eq-x", sort)
		if cause != nil {
			return nil, false, cause
		}
		ref, cause := p.manager.NewRef(auxiliary)
		if cause != nil {
			return nil, false, cause
		}
		freshRefs = append(freshRefs, ref)
		transformed.opaque = append(transformed.opaque, arithmeticEquationTerm{new(big.Rat).SetInt(row[rowIndex]), auxiliary})
	}
	definitionRef, err := p.equationRest(transformed, variables, index, variables[index])
	if err != nil {
		for i := range freshRefs {
			_ = p.manager.DelRef(&freshRefs[i])
		}
		return nil, false, err
	}
	definition, err := p.manager.RefGet(definitionRef)
	if err != nil {
		_ = p.manager.DelRef(&definitionRef)
		for i := range freshRefs {
			_ = p.manager.DelRef(&freshRefs[i])
		}
		return nil, false, err
	}
	elimination, err := p.rewriteEquation(index, variables[index], definition, formula, freshRefs)
	closeErr := p.manager.DelRef(&definitionRef)
	if closeErr != nil && elimination != nil {
		closeErr = errors.Join(closeErr, elimination.Close())
		elimination = nil
	}
	err = errors.Join(err, closeErr)
	if err != nil && elimination == nil {
		for i := range freshRefs {
			if freshRefs[i] != (ast.ASTRef{}) {
				err = errors.Join(err, p.manager.DelRef(&freshRefs[i]))
			}
		}
	}
	return elimination, err == nil, err
}
