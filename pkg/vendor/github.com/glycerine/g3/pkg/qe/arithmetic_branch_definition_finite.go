package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"math/big"
)

func (p *ArithmeticPlugin) finiteBranchDefinition(x ArithmeticBranchSelection, s *arithmeticBranchSubstitution) (ast.ASTRef, error) {
	if x.Bound == nil {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	t, e := p.manager.RefGet(x.Bound.Term)
	if e != nil {
		return ast.ASTRef{}, e
	}
	t, a, e := p.adjustDefinition(t, x.Bound.Coefficient, s)
	if e != nil {
		return ast.ASTRef{}, e
	}
	d, e := p.definitionDivision(t, a)
	if e != nil {
		return ast.ASTRef{}, e
	}
	if x.Kind == ArithmeticStrictBranch {
		o, e := p.util.numeralFor(big.NewRat(1, 1), d)
		if e != nil {
			return ast.ASTRef{}, e
		}
		d, e = p.manager.MkArithmeticSub(d, o)
		if e != nil {
			return ast.ASTRef{}, e
		}
	}
	if s != nil {
		scaled, cause := p.util.scale(s.coefficient, d)
		if cause != nil {
			return ast.ASTRef{}, cause
		}
		h, cause := p.manager.RefGet(scaled)
		if cause == nil {
			d, cause = p.manager.MkArithmeticAdd(h, s.term)
		}
		cause = errors.Join(cause, p.manager.DelRef(&scaled))
		if cause != nil {
			return ast.ASTRef{}, cause
		}
	}
	return p.manager.NewRef(d)
}
