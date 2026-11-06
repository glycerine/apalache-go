package qe

import "github.com/glycerine/g3/pkg/ast"

func (p *ArithmeticPlugin) infinityCombine(target ast.ASTHandle, terms []ast.ASTHandle, lower bool) (ast.ASTRef, error) {
	if len(terms) == 0 {
		z, e := p.util.zero(target)
		if e != nil {
			return ast.ASTRef{}, e
		}
		return p.manager.NewRef(z)
	}
	if lower {
		return p.util.MakeMin(terms)
	}
	return p.util.MakeMax(terms)
}
