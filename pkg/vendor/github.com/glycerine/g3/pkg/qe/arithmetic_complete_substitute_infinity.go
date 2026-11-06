package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

func (p *ArithmeticPlugin) substituteCompleteInfinity(variable *ast.ContainsApp, root ast.ASTHandle, formula ast.ASTRef, definition *ast.ASTRef) (result error) {
	d, e := p.InfinityDefinition(variable, root)
	if e != nil {
		return e
	}
	defer func() { result = errors.Join(result, p.manager.DelRef(&d)) }()
	if e = p.SubstituteBranchZero(variable, root, formula); e != nil {
		return e
	}
	return p.setCompleteDefinition(definition, d)
}
