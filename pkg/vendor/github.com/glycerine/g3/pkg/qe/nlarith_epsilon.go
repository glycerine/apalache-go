package qe

import "github.com/glycerine/g3/pkg/ast"

func (u *NonlinearArithmeticUtil) plusEpsilonLT(p *NLPolynomial, root NLRootForm) (ast.ASTHandle, error) {
	less, err := u.RootCompare(p, root, NLComparisonLT)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if p.Len() <= 1 {
		return less, nil
	}
	equal, err := u.RootCompare(p, root, NLComparisonEQ)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	derivative, err := u.Derivative(p)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	next, err := u.plusEpsilonLT(derivative, root)
	closeErr := derivative.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	tail, err := u.and(equal, next)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return u.or(less, tail)
}
func (u *NonlinearArithmeticUtil) PlusEpsilonCompare(p *NLPolynomial, root NLRootForm, comparison NLComparison) (ast.ASTHandle, error) {
	switch comparison {
	case NLComparisonLT:
		return u.plusEpsilonLT(p, root)
	case NLComparisonEQ:
		return u.polynomialZero(p)
	case NLComparisonNE:
		eq, err := u.polynomialZero(p)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return u.manager.MkNot(eq)
	case NLComparisonLE:
		lt, err := u.plusEpsilonLT(p, root)
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
