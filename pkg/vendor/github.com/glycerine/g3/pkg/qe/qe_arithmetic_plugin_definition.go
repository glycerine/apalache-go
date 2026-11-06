package qe

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// boundDefinition solves a*x+t=0 for x. It is the finite-branch definition
// retained by subst and projection.
func (p *ArithmeticPlugin) boundDefinition(bound *ArithmeticBound) (ast.ASTRef, error) {
	if bound == nil || bound.Coefficient == nil || bound.Coefficient.Sign() == 0 {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	t, err := p.manager.RefGet(bound.Term)
	if err != nil {
		return ast.ASTRef{}, err
	}
	negative, err := p.manager.MkArithmeticNeg(t)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if bound.Coefficient.Cmp(big.NewRat(1, 1)) == 0 {
		return p.manager.NewRef(negative)
	}
	denominator, err := p.util.numeralFor(bound.Coefficient, t)
	if err != nil {
		return ast.ASTRef{}, err
	}
	division, err := p.manager.MkArithmeticDiv(negative, denominator)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.manager.NewRef(division)
}
