package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type ArithmeticConstraint struct {
	UseVariable bool
	Formula     ast.ASTRef
}

func (p *ArithmeticPlugin) appendConstraint(useVariable bool, formula ast.ASTHandle) error {
	r, err := p.manager.NewRef(formula)
	if err != nil {
		return err
	}
	p.constraints = append(p.constraints, ArithmeticConstraint{useVariable, r})
	return nil
}

func (p *ArithmeticPlugin) clearConstraints() (result error) {
	for i := len(p.constraints) - 1; i >= 0; i-- {
		result = errors.Join(result, p.manager.DelRef(&p.constraints[i].Formula))
	}
	p.constraints = nil
	return result
}

func (p *ArithmeticPlugin) TakeConstraints() ([]ArithmeticConstraint, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	result := p.constraints
	p.constraints = nil
	return result, nil
}
