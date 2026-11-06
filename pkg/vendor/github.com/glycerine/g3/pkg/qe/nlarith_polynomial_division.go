package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"math/big"
)

func (u *NonlinearArithmeticUtil) NumericQuotientRemainder(dividend, divisor *NLPolynomial) (quotient, remainder *NLPolynomial, handled bool, resultErr error) {
	if err := u.open(); err != nil {
		return nil, nil, false, err
	}
	a, err := dividend.handles()
	if err != nil {
		return nil, nil, false, err
	}
	b, err := divisor.handles()
	if err != nil {
		return nil, nil, false, err
	}
	if len(b) == 0 {
		return nil, nil, false, ast.ErrASTDomain
	}
	lead, numeral, err := u.Numeral(b[len(b)-1])
	if err != nil || !numeral || lead.Sign() == 0 {
		return nil, nil, false, err
	}
	if len(a) < len(b) {
		r, err := newNLPolynomial(u.manager, a)
		q, qe := newNLPolynomial(u.manager, nil)
		if err == nil {
			err = qe
		}
		return q, r, true, err
	}
	z, _, err := u.zeroOne(a[0])
	if err != nil {
		return nil, nil, false, err
	}
	q := make([]ast.ASTHandle, len(a)-len(b)+1)
	for i := range q {
		q[i] = z
	}
	r := append([]ast.ASTHandle(nil), a...)
	inverse := new(big.Rat).Inv(lead)
	inv, err := u.term.numeralFor(inverse, b[len(b)-1])
	if err != nil {
		return nil, nil, false, err
	}
	for k := len(q) - 1; k >= 0; k-- {
		q[k], err = u.manager.MkArithmeticMul(r[len(b)-1+k], inv)
		if err != nil {
			return nil, nil, false, err
		}
		for j := len(b) - 2 + k; j >= k; j-- {
			product, cause := u.manager.MkArithmeticMul(q[k], b[j-k])
			if cause != nil {
				return nil, nil, false, cause
			}
			r[j], cause = u.manager.MkArithmeticSub(r[j], product)
			if cause != nil {
				return nil, nil, false, cause
			}
		}
	}
	quotient, err = newNLPolynomial(u.manager, q)
	if err != nil {
		return nil, nil, false, err
	}
	remainder, err = newNLPolynomial(u.manager, r[:len(b)-1])
	if err != nil {
		_ = quotient.Close()
		return nil, nil, false, err
	}
	return quotient, remainder, true, nil
}
