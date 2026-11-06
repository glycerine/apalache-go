package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (p *ArithmeticPlugin) ValidateCompleteProjection(variable *ast.ContainsApp, source *model.Model, formula ast.ASTRef) error {
	if variable == nil || source == nil {
		return ast.ErrASTHandle
	}
	_, e := p.manager.RefGet(formula)
	return e
}
