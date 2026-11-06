package qe

import "github.com/glycerine/g3/pkg/ast"

func (p *ArithmeticPlugin) infinityNonStrict(b *ArithmeticBound) (ast.ASTHandle, error) {
	if b == nil {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	t, e := p.manager.RefGet(b.Term)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return p.definitionDivision(t, b.Coefficient)
}
