package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (p *NonlinearArithmeticPlugin) Assign(variable *ast.ContainsApp, formula ast.ASTHandle, value *numeral.OwnedRational) error {
	if value == nil {
		return ast.ErrASTHandle
	}
	e, _, err := p.entry(variable, formula)
	if err != nil {
		return err
	}
	branch, err := p.manager.NumeralService().Unsigned(value)
	if err != nil || int(branch) >= len(e.branches.Branches) {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return err
	}
	h, err := p.manager.RefGet(e.branches.Branches[branch].Branch)
	if err != nil {
		return err
	}
	r, err := p.manager.NewRef(h)
	if err != nil {
		return err
	}
	p.constraints = append(p.constraints, ArithmeticConstraint{UseVariable: true, Formula: r})
	return nil
}
func (p *NonlinearArithmeticPlugin) TakeConstraints() ([]ArithmeticConstraint, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	r := p.constraints
	p.constraints = nil
	return r, nil
}
