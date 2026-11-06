package qe

import "github.com/glycerine/g3/pkg/ast"

func (p *ArithmeticPlugin) PrepareFormulaNNF(variable *ast.ContainsApp, formula ast.ASTHandle) (bool, error) {
	atoms, e := CollectArithmeticAtomsNNF(p.manager, formula)
	if e != nil {
		return false, e
	}
	defer atoms.Close()
	return p.PrepareAtoms(variable, formula, atoms)
}
