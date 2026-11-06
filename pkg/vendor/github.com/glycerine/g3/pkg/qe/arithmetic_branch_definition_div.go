package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"math/big"
)

func (p *ArithmeticPlugin) definitionDivision(t ast.ASTHandle, a *big.Rat) (ast.ASTHandle, error) {
	n, e := p.manager.MkArithmeticNeg(t)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	if a.Cmp(big.NewRat(1, 1)) == 0 {
		return n, nil
	}
	d, e := p.util.numeralFor(a, t)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return p.manager.MkArithmeticDiv(n, d)
}
