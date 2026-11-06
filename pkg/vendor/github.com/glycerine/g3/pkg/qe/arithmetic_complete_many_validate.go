package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (p *ArithmeticPlugin) validateCompleteMany(variables []ast.ASTHandle, source *model.Model, formula ast.ASTRef) error {
	if source == nil {
		return ast.ErrASTHandle
	}
	if _, e := p.manager.RefGet(formula); e != nil {
		return e
	}
	for _, v := range variables {
		k, e := p.manager.GetKind(v)
		if e != nil || k != ast.ASTApp {
			return ast.ErrASTDomain
		}
	}
	return nil
}
