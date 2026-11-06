package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (p *ArithmeticPlugin) ProjectModelFormulaNNFComplete(variable *ast.ContainsApp, source *model.Model, formula ast.ASTRef, definition *ast.ASTRef, config Config) (projected bool, resultErr error) {
	if e := p.open(); e != nil {
		return false, e
	}
	c, e := p.newCompleteProject(variable, source, formula, config)
	if e != nil {
		return false, e
	}
	defer func() { resultErr = errors.Join(resultErr, c.close()) }()
	branch, e := c.selectBranch()
	if e != nil {
		return false, e
	}
	if e = c.apply(formula, definition, branch); e != nil {
		return false, e
	}
	return true, nil
}
