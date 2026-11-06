package qe

import "github.com/glycerine/g3/pkg/ast"

func (u *NonlinearArithmeticUtil) RootCompare(p *NLPolynomial, root NLRootForm, comparison NLComparison) (ast.ASTHandle, error) {
	ar, br, dr, err := u.instantiate(p, root)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ad, bd := ar, br
	if p.Len()%2 == 0 {
		ad, err = u.manager.MkArithmeticMul(ar, dr)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		bd, err = u.manager.MkArithmeticMul(br, dr)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	if root.Multiplier == 0 {
		return u.zeroComparison(ad, comparison)
	}
	aa, err := u.manager.MkArithmeticMul(ar, ar)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bb, err := u.manager.MkArithmeticMul(br, br)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bbc, err := u.manager.MkArithmeticMul(bb, root.C)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	delta, err := u.manager.MkArithmeticSub(aa, bbc)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if comparison == NLComparisonEQ {
		ab, er := u.manager.MkArithmeticMul(ar, br)
		if er != nil {
			return ast.ASTHandle{}, er
		}
		abLE, er := u.zeroComparison(ab, NLComparisonLE)
		if er != nil {
			return ast.ASTHandle{}, er
		}
		dEQ, er := u.zeroComparison(delta, NLComparisonEQ)
		if er != nil {
			return ast.ASTHandle{}, er
		}
		return u.and(abLE, dEQ)
	}
	if comparison == NLComparisonNE {
		eq, er := u.RootCompare(p, root, NLComparisonEQ)
		if er != nil {
			return ast.ASTHandle{}, er
		}
		return u.manager.MkNot(eq)
	}
	adCmp, err := u.zeroComparison(ad, comparison)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bdLE, err := u.zeroComparison(bd, NLComparisonLE)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if comparison == NLComparisonLT {
		dGT, er := u.manager.MkArithmeticGT(delta, mustArithmeticZero(u, delta))
		if er != nil {
			return ast.ASTHandle{}, er
		}
		dLT, er := u.zeroComparison(delta, NLComparisonLT)
		if er != nil {
			return ast.ASTHandle{}, er
		}
		left, er := u.and(adCmp, dGT)
		if er != nil {
			return ast.ASTHandle{}, er
		}
		inner, er := u.or(adCmp, dLT)
		if er != nil {
			return ast.ASTHandle{}, er
		}
		right, er := u.and(bdLE, inner)
		if er != nil {
			return ast.ASTHandle{}, er
		}
		return u.or(left, right)
	}
	dGE, err := u.manager.MkArithmeticGE(delta, mustArithmeticZero(u, delta))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	dLE, err := u.zeroComparison(delta, NLComparisonLE)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err := u.and(adCmp, dGE)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := u.and(bdLE, dLE)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return u.or(left, right)
}
func mustArithmeticZero(u *NonlinearArithmeticUtil, h ast.ASTHandle) ast.ASTHandle {
	z, _ := u.term.zero(h)
	return z
}
