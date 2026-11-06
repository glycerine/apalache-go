package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (p *ArithmeticPlugin) SubstituteBranchZero(variable *ast.ContainsApp, root ast.ASTHandle, formula ast.ASTRef) error {
	s := p.manager.NumeralService()
	v, e := s.NewValue()
	if e != nil {
		return e
	}
	defer s.Del(&v)
	if e = s.SetUnsigned(&v, 0); e != nil {
		return e
	}
	return p.Substitute(variable, (*numeral.OwnedRational)(&v), formula, nil)
}
