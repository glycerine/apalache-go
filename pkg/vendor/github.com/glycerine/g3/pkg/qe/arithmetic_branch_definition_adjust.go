package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"math/big"
)

func (p *ArithmeticPlugin) adjustDefinition(t ast.ASTHandle, a *big.Rat, s *arithmeticBranchSubstitution) (ast.ASTHandle, *big.Rat, error) {
	if s == nil {
		return t, new(big.Rat).Set(a), nil
	}
	r, e := p.util.scale(a, s.term)
	if e != nil {
		return ast.ASTHandle{}, nil, e
	}
	h, e := p.manager.RefGet(r)
	if e != nil {
		_ = p.manager.DelRef(&r)
		return ast.ASTHandle{}, nil, e
	}
	v, e := p.manager.MkArithmeticAdd(t, h)
	e = errors.Join(e, p.manager.DelRef(&r))
	return v, new(big.Rat).Mul(a, s.coefficient), e
}
