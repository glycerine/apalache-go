package qe

import "github.com/glycerine/g3/pkg/ast"

func (p *ArithmeticPlugin) InfinityDefinition(variable *ast.ContainsApp, formula ast.ASTHandle) (ast.ASTRef, error) {
	entry, target, e := p.entry(variable, formula)
	if e != nil {
		return ast.ASTRef{}, e
	}
	policy, e := p.branchPolicy(entry, target)
	if e != nil {
		return ast.ASTRef{}, e
	}
	return p.infinityDefinition(entry, target, policy)
}
