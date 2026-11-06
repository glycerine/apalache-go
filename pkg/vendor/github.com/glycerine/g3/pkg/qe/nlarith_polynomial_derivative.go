package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"strconv"
)

func (u *NonlinearArithmeticUtil) Derivative(p *NLPolynomial) (*NLPolynomial, error) {
	if err := u.open(); err != nil {
		return nil, err
	}
	h, err := p.handles()
	if err != nil {
		return nil, err
	}
	if len(h) == 0 {
		return newNLPolynomial(u.manager, nil)
	}
	if len(h) == 1 {
		z, _, err := u.zeroOne(h[0])
		if err != nil {
			return nil, err
		}
		return newNLPolynomial(u.manager, []ast.ASTHandle{z})
	}
	out := make([]ast.ASTHandle, len(h)-1)
	for i := 1; i < len(h); i++ {
		integer, err := u.term.isInteger(h[i])
		if err != nil {
			return nil, err
		}
		var n ast.ASTHandle
		if integer {
			n, err = u.manager.ArithmeticIntegerNumeralText(strconv.Itoa(i))
		} else {
			n, err = u.manager.ArithmeticRealNumeralText(strconv.Itoa(i))
		}
		if err != nil {
			return nil, err
		}
		out[i-1], err = u.manager.MkArithmeticMul(n, h[i])
		if err != nil {
			return nil, err
		}
	}
	return newNLPolynomial(u.manager, out)
}
