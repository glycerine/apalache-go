package qe

import "github.com/glycerine/g3/pkg/ast"

func (u *NonlinearArithmeticUtil) realNumeral(text string) (ast.ASTHandle, error) {
	return u.manager.ArithmeticRealNumeralText(text)
}
func (u *NonlinearArithmeticUtil) zeroComparison(e ast.ASTHandle, kind NLComparison) (ast.ASTHandle, error) {
	z, err := u.term.zero(e)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	switch kind {
	case NLComparisonLT:
		return u.manager.MkArithmeticLT(e, z)
	case NLComparisonLE:
		return u.manager.MkArithmeticLE(e, z)
	case NLComparisonEQ:
		return u.manager.MkEq(e, z)
	case NLComparisonNE:
		eq, err := u.manager.MkEq(e, z)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return u.manager.MkNot(eq)
	}
	return ast.ASTHandle{}, ast.ErrASTDomain
}
func (u *NonlinearArithmeticUtil) and(args ...ast.ASTHandle) (ast.ASTHandle, error) {
	return u.manager.MkAndArray(args)
}
func (u *NonlinearArithmeticUtil) or(args ...ast.ASTHandle) (ast.ASTHandle, error) {
	return u.manager.MkOrArray(args)
}
func (u *NonlinearArithmeticUtil) definitelyZero(e ast.ASTHandle) (bool, error) {
	v, ok, err := u.Numeral(e)
	return ok && v.Sign() == 0, err
}
func (u *NonlinearArithmeticUtil) coefficients(p *NLPolynomial) (a, b, c ast.ASTHandle, err error) {
	h, err := p.handles()
	if err != nil {
		return a, b, c, err
	}
	witness := h[0]
	z, _, err := u.zeroOne(witness)
	if err != nil {
		return a, b, c, err
	}
	a, b, c = z, z, z
	if len(h) > 0 {
		c = h[0]
	}
	if len(h) > 1 {
		b = h[1]
	}
	if len(h) > 2 {
		a = h[2]
	}
	if len(h) > 3 {
		return a, b, c, ast.ErrASTDomain
	}
	return
}
func (u *NonlinearArithmeticUtil) polynomialZero(p *NLPolynomial) (ast.ASTHandle, error) {
	h, err := p.handles()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	equalities := make([]ast.ASTHandle, len(h))
	for i := range h {
		equalities[i], err = u.zeroComparison(h[i], NLComparisonEQ)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return u.and(equalities...)
}
