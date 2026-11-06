package qe

import "github.com/glycerine/g3/pkg/ast"

func (p *ArithmeticPlugin) InfinityDefinitionDirection(variable *ast.ContainsApp, formula ast.ASTHandle) (ArithmeticBoundDirection, error) {
	entry, target, e := p.entry(variable, formula)
	if e != nil {
		return 0, e
	}
	policy, e := p.branchPolicy(entry, target)
	if e != nil {
		return 0, e
	}
	if policy.Lower {
		return ArithmeticUpperBound, nil
	}
	return ArithmeticLowerBound, nil
}
