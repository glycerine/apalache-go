package qe

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *ArithmeticPlugin) parseEquation(expression ast.ASTHandle, variables []ast.ASTHandle) (*arithmeticEquation, error) {
	equation := &arithmeticEquation{constant: new(big.Rat), coefficients: make([]*big.Rat, len(variables))}
	for index := range equation.coefficients {
		equation.coefficients[index] = new(big.Rat)
	}
	if err := p.parseEquationTerm(equation, expression, big.NewRat(1, 1), variables); err != nil {
		return nil, err
	}
	return equation, nil
}

func (p *ArithmeticPlugin) parseEquationTerm(equation *arithmeticEquation, expression ast.ASTHandle, factor *big.Rat, variables []ast.ASTHandle) error {
	if value, numeral, err := p.util.numeral(expression); err != nil {
		return err
	} else if numeral {
		equation.constant.Add(equation.constant, new(big.Rat).Mul(factor, value))
		return nil
	}
	for index, variable := range variables {
		if expression == variable {
			equation.coefficients[index].Add(equation.coefficients[index], factor)
			return nil
		}
	}
	kind, err := p.manager.GetKind(expression)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		equation.opaque = append(equation.opaque, arithmeticEquationTerm{new(big.Rat).Set(factor), expression})
		return nil
	}
	family, err := p.manager.GetFamilyID(expression)
	if err != nil {
		return err
	}
	operation, err := p.manager.GetDeclKind(expression)
	if err != nil {
		return err
	}
	if family == p.family {
		switch operation {
		case 6:
			count, cause := p.manager.GetNumArgs(expression)
			if cause != nil {
				return cause
			}
			for index := uint32(0); index < count; index++ {
				argument, cause := p.manager.GetArg(expression, index)
				if cause != nil {
					return cause
				}
				if cause = p.parseEquationTerm(equation, argument, factor, variables); cause != nil {
					return cause
				}
			}
			return nil
		case 7:
			left, cause := p.manager.GetArg(expression, 0)
			if cause != nil {
				return cause
			}
			right, cause := p.manager.GetArg(expression, 1)
			if cause != nil {
				return cause
			}
			if cause = p.parseEquationTerm(equation, left, factor, variables); cause != nil {
				return cause
			}
			return p.parseEquationTerm(equation, right, new(big.Rat).Neg(factor), variables)
		case 8:
			argument, cause := p.manager.GetArg(expression, 0)
			if cause != nil {
				return cause
			}
			return p.parseEquationTerm(equation, argument, new(big.Rat).Neg(factor), variables)
		case 9:
			left, cause := p.manager.GetArg(expression, 0)
			if cause != nil {
				return cause
			}
			right, cause := p.manager.GetArg(expression, 1)
			if cause != nil {
				return cause
			}
			if value, numeral, cause := p.util.numeral(left); cause != nil {
				return cause
			} else if numeral {
				return p.parseEquationTerm(equation, right, new(big.Rat).Mul(factor, value), variables)
			}
			if value, numeral, cause := p.util.numeral(right); cause != nil {
				return cause
			} else if numeral {
				return p.parseEquationTerm(equation, left, new(big.Rat).Mul(factor, value), variables)
			}
		}
	}
	equation.opaque = append(equation.opaque, arithmeticEquationTerm{new(big.Rat).Set(factor), expression})
	return nil
}
