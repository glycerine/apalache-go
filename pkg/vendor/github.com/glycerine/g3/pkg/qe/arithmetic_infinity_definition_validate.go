package qe

import "github.com/glycerine/g3/pkg/ast"

func (p *ArithmeticPlugin) ValidateInfinityDefinition(variable *ast.ContainsApp, formula ast.ASTHandle) error {
	if variable == nil {
		return ast.ErrASTHandle
	}
	_, _, e := p.entry(variable, formula)
	return e
}
