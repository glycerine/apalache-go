package qe

import "github.com/glycerine/g3/pkg/ast"

func (u *NonlinearArithmeticUtil) combine(left, right ast.ASTHandle, kind int32) (ast.ASTHandle, error) {
	switch kind {
	case 6:
		return u.manager.MkArithmeticAdd(left, right)
	case 7:
		return u.manager.MkArithmeticSub(left, right)
	case 9:
		return u.manager.MkArithmeticMul(left, right)
	}
	return ast.ASTHandle{}, ast.ErrASTDomain
}
func (u *NonlinearArithmeticUtil) Negate(p *NLPolynomial) error {
	if err := u.open(); err != nil {
		return err
	}
	h, err := p.handles()
	if err != nil {
		return err
	}
	for i := range h {
		h[i], err = u.manager.MkArithmeticNeg(h[i])
		if err != nil {
			return err
		}
	}
	return p.replace(h)
}
func (u *NonlinearArithmeticUtil) addSub(p, q *NLPolynomial, subtract bool) error {
	if err := u.open(); err != nil {
		return err
	}
	a, err := p.handles()
	if err != nil {
		return err
	}
	b, err := q.handles()
	if err != nil {
		return err
	}
	z, _, err := u.zeroOne(a[0])
	if err != nil {
		return err
	}
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	out := make([]ast.ASTHandle, n)
	for i := 0; i < n; i++ {
		x, y := z, z
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if subtract {
			out[i], err = u.manager.MkArithmeticSub(x, y)
		} else {
			out[i], err = u.manager.MkArithmeticAdd(x, y)
		}
		if err != nil {
			return err
		}
	}
	return p.replace(out)
}
func (u *NonlinearArithmeticUtil) Add(p, q *NLPolynomial) error      { return u.addSub(p, q, false) }
func (u *NonlinearArithmeticUtil) Subtract(p, q *NLPolynomial) error { return u.addSub(p, q, true) }
func (u *NonlinearArithmeticUtil) Multiply(p, q *NLPolynomial) error {
	if err := u.open(); err != nil {
		return err
	}
	a, err := p.handles()
	if err != nil {
		return err
	}
	b, err := q.handles()
	if err != nil {
		return err
	}
	if len(a) == 0 || len(b) == 0 {
		return p.replace(nil)
	}
	z, _, err := u.zeroOne(a[0])
	if err != nil {
		return err
	}
	out := make([]ast.ASTHandle, len(a)+len(b)-1)
	for i := range out {
		out[i] = z
	}
	for i := range a {
		for j := range b {
			product, cause := u.manager.MkArithmeticMul(a[i], b[j])
			if cause != nil {
				return cause
			}
			out[i+j], cause = u.manager.MkArithmeticAdd(out[i+j], product)
			if cause != nil {
				return cause
			}
		}
	}
	return p.replace(out)
}
func (u *NonlinearArithmeticUtil) Scale(p *NLPolynomial, factor ast.ASTHandle) error {
	h, err := p.handles()
	if err != nil {
		return err
	}
	for i := range h {
		h[i], err = u.manager.MkArithmeticMul(h[i], factor)
		if err != nil {
			return err
		}
	}
	return p.replace(h)
}
