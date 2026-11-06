package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"math/big"
)

func (p *ArithmeticPlugin) infinityStrict(b *ArithmeticBound, target ast.ASTHandle, lower bool) (ast.ASTHandle, error) {
	t, e := p.manager.RefGet(b.Term)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	one, e := p.util.numeralFor(big.NewRat(1, 1), target)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	t, e = p.manager.MkArithmeticAdd(t, one)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	if lower {
		return p.manager.MkArithmeticNeg(t)
	}
	return t, nil
}
