package qe

import "github.com/glycerine/g3/pkg/ast"

func (p *ArithmeticPlugin) setCompleteDefinition(target *ast.ASTRef, source ast.ASTRef) error {
	if target == nil {
		return nil
	}
	if source == (ast.ASTRef{}) {
		return p.manager.SetRef(*target, ast.ASTHandle{})
	}
	h, e := p.manager.RefGet(source)
	if e != nil {
		return e
	}
	return p.manager.SetRef(*target, h)
}
