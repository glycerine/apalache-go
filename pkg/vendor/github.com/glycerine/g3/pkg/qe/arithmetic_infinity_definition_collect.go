package qe

import "github.com/glycerine/g3/pkg/ast"

func (p *ArithmeticPlugin) infinityDefinition(entry *arithmeticPluginBounds, target ast.ASTHandle, policy ArithmeticBranchPolicy) (ast.ASTRef, error) {
	direction := ArithmeticLowerBound
	if policy.Lower {
		direction = ArithmeticUpperBound
	}
	var terms []ast.ASTHandle
	for _, b := range entry.processor.Bounds(false, direction) {
		h, e := p.infinityNonStrict(b)
		if e != nil {
			return ast.ASTRef{}, e
		}
		terms = append(terms, h)
	}
	for _, b := range entry.processor.Bounds(true, direction) {
		h, e := p.infinityStrict(b, target, policy.Lower)
		if e != nil {
			return ast.ASTRef{}, e
		}
		terms = append(terms, h)
	}
	return p.infinityCombine(target, terms, policy.Lower)
}
