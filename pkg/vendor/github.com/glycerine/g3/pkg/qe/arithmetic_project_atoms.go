package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (p *ArithmeticPlugin) ProjectModelAtoms(variable *ast.ContainsApp, source *model.Model, formula ast.ASTRef, atoms *ArithmeticAtomSet, config Config) (bool, error) {
	root, e := p.manager.RefGet(formula)
	if e != nil {
		return false, e
	}
	ok, e := p.PrepareAtoms(variable, root, atoms)
	if e != nil || !ok {
		return false, e
	}
	return p.ProjectModel(variable, source, formula, config)
}
