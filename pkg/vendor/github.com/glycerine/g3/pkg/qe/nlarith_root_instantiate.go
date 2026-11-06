package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"strconv"
)

// instantiate returns AR + BR*sqrt(C) over DR using source Horner order.
func (u *NonlinearArithmeticUtil) instantiate(p *NLPolynomial, root NLRootForm) (ar, br, dr ast.ASTHandle, err error) {
	h, err := p.handles()
	if err != nil {
		return ar, br, dr, err
	}
	z, o, err := u.zeroOne(root.A)
	if err != nil {
		return ar, br, dr, err
	}
	br, dr = z, o
	if len(h) == 0 {
		return z, br, dr, nil
	}
	ar = h[len(h)-1]
	b, err := u.realNumeral(strconv.Itoa(root.Multiplier))
	if err != nil {
		return ar, br, dr, err
	}
	for i := len(h) - 2; i >= 0; i-- {
		oldAR := ar
		dd, er := u.manager.MkArithmeticMul(root.D, dr)
		if er != nil {
			return ar, br, dr, er
		}
		term0, er := u.manager.MkArithmeticMul(dd, h[i])
		if er != nil {
			return ar, br, dr, er
		}
		term1, er := u.manager.MkArithmeticMul(root.A, ar)
		if er != nil {
			return ar, br, dr, er
		}
		bb, er := u.manager.MkArithmeticMul(b, br)
		if er != nil {
			return ar, br, dr, er
		}
		term2, er := u.manager.MkArithmeticMul(bb, root.C)
		if er != nil {
			return ar, br, dr, er
		}
		sum, er := u.manager.MkArithmeticAdd(term0, term1)
		if er != nil {
			return ar, br, dr, er
		}
		ar, er = u.manager.MkArithmeticAdd(sum, term2)
		if er != nil {
			return ar, br, dr, er
		}
		abr, er := u.manager.MkArithmeticMul(root.A, br)
		if er != nil {
			return ar, br, dr, er
		}
		arb, er := u.manager.MkArithmeticMul(oldAR, b)
		if er != nil {
			return ar, br, dr, er
		}
		br, er = u.manager.MkArithmeticAdd(abr, arb)
		if er != nil {
			return ar, br, dr, er
		}
		dr = dd
	}
	return
}

var _ ast.ASTHandle
