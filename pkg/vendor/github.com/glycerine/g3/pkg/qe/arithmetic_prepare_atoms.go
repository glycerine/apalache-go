package qe

import "github.com/glycerine/g3/pkg/ast"

func (p *ArithmeticPlugin) PrepareAtoms(variable *ast.ContainsApp, formula ast.ASTHandle, atoms *ArithmeticAtomSet) (bool, error) {
	if atoms == nil || atoms.manager != p.manager {
		return false, ast.ErrASTHandle
	}
	positive, e := atoms.Positive()
	if e != nil {
		return false, e
	}
	negative, e := atoms.Negative()
	if e != nil {
		return false, e
	}
	return p.Prepare(variable, formula, positive, negative)
}
