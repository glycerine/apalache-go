package qe

import "github.com/glycerine/g3/pkg/ast"

func (p *ArithmeticPlugin) validateContextVariables(variables []ast.ASTHandle) error {
	for _, v := range variables {
		sort, e := p.manager.GetSort(v)
		if e != nil {
			return e
		}
		family, e := p.manager.GetFamilyID(sort)
		if e != nil {
			return e
		}
		if family != p.family {
			return ast.ErrASTDomain
		}
	}
	return nil
}
