package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// AssignSubstituteComplete publishes the selected result and its complete witness definition.
func (p *ArithmeticPlugin) AssignSubstituteComplete(variable *ast.ContainsApp, formula ast.ASTRef, value *numeral.OwnedRational, definition *ast.ASTRef) error {
	root, e := p.manager.RefGet(formula)
	if e != nil {
		return e
	}
	branch, e := p.completeBranchValue(value)
	if e != nil {
		return e
	}
	if e = p.Assign(variable, root, value); e != nil {
		return e
	}
	if branch == 0 {
		return p.substituteCompleteInfinity(variable, root, formula, definition)
	}
	return p.substituteCompleteFinite(variable, value, formula, definition)
}
