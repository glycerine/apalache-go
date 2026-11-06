package qe

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *ArithmeticPlugin) solveSingularEquation(equation *arithmeticEquation, variables []ast.ASTHandle, formula ast.ASTHandle) (*ArithmeticEquationElimination, bool, error) {
	one := big.NewRat(1, 1)
	for index, variable := range variables {
		coefficient := equation.coefficients[index]
		if coefficient.Sign() == 0 {
			continue
		}
		blocked := false
		contains, err := ast.NewContainsApp(p.manager, variable)
		if err != nil {
			return nil, false, err
		}
		for _, term := range equation.opaque {
			found, cause := contains.Check(term.expression)
			if cause != nil {
				err = cause
				break
			}
			blocked = blocked || found
		}
		err = errors.Join(err, contains.Close())
		if err != nil {
			return nil, false, err
		}
		if blocked {
			continue
		}
		integer, err := p.util.isInteger(variable)
		if err != nil {
			return nil, false, err
		}
		absolute := new(big.Rat).Abs(coefficient)
		if integer && absolute.Cmp(one) != 0 {
			continue
		}
		rest, err := p.equationRest(equation, variables, index, variable)
		if err != nil {
			return nil, false, err
		}
		restHandle, err := p.manager.RefGet(rest)
		if err != nil {
			_ = p.manager.DelRef(&rest)
			return nil, false, err
		}
		definition := restHandle
		switch coefficient.Cmp(one) {
		case 0:
			definition, err = p.manager.MkArithmeticNeg(restHandle)
		default:
			if coefficient.Cmp(big.NewRat(-1, 1)) != 0 {
				negative, cause := p.manager.MkArithmeticNeg(restHandle)
				if cause == nil {
					var divisor ast.ASTHandle
					divisor, cause = p.util.numeralFor(coefficient, variable)
					if cause == nil {
						definition, cause = p.manager.MkArithmeticDiv(negative, divisor)
					}
				}
				err = cause
			}
		}
		if err != nil {
			_ = p.manager.DelRef(&rest)
			return nil, false, err
		}
		elimination, err := p.rewriteEquation(index, variable, definition, formula, nil)
		err = errors.Join(err, p.manager.DelRef(&rest))
		return elimination, err == nil, err
	}
	return nil, false, nil
}
