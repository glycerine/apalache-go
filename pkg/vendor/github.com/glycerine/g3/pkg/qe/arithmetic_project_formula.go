package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (p *ArithmeticPlugin) ProjectModelFormulaNNF(variable *ast.ContainsApp, source *model.Model, formula ast.ASTRef, config Config) (bool, error) {
	root, e := p.manager.RefGet(formula)
	if e != nil {
		return false, e
	}
	atoms, e := CollectArithmeticAtomsNNF(p.manager, root)
	if e != nil {
		return false, e
	}
	defer atoms.Close()
	return p.ProjectModelAtoms(variable, source, formula, atoms, config)
}
