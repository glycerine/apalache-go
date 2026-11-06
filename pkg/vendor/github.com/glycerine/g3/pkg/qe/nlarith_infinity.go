package qe

import "github.com/glycerine/g3/pkg/ast"

func (u *NonlinearArithmeticUtil) minusInfinityLT(p *NLPolynomial, index int) (ast.ASTHandle, error) {
	if index == 0 {
		return u.manager.MkFalse()
	}
	index--
	coefficient, err := p.Coefficient(uint32(index))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var sign ast.ASTHandle
	if index%2 == 0 {
		sign, err = u.zeroComparison(coefficient, NLComparisonLT)
	} else {
		z, cause := u.term.zero(coefficient)
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		sign, err = u.manager.MkArithmeticGT(coefficient, z)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if index == 0 {
		return sign, nil
	}
	zero, err := u.zeroComparison(coefficient, NLComparisonEQ)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rest, err := u.minusInfinityLT(p, index)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	tail, err := u.and(zero, rest)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return u.or(sign, tail)
}
func (u *NonlinearArithmeticUtil) MinusInfinityCompare(p *NLPolynomial, comparison NLComparison) (ast.ASTHandle, error) {
	switch comparison {
	case NLComparisonLT:
		return u.minusInfinityLT(p, p.Len())
	case NLComparisonEQ:
		return u.polynomialZero(p)
	case NLComparisonNE:
		eq, err := u.polynomialZero(p)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return u.manager.MkNot(eq)
	case NLComparisonLE:
		lt, err := u.minusInfinityLT(p, p.Len())
		if err != nil {
			return ast.ASTHandle{}, err
		}
		eq, err := u.polynomialZero(p)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return u.or(lt, eq)
	}
	return ast.ASTHandle{}, ast.ErrASTDomain
}
