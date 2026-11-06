package qe

import "github.com/glycerine/g3/pkg/ast"

type nlCandidate struct {
	root       NLRootForm
	condition  ast.ASTHandle
	definition ast.ASTHandle
	a, b, c    ast.ASTHandle
	strict     bool
}

func (u *NonlinearArithmeticUtil) rootDefinition(root NLRootForm, comparison NLComparison, a, b ast.ASTHandle) (ast.ASTHandle, error) {
	base, err := u.RootExpression(root)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if comparison != NLComparisonLT && comparison != NLComparisonNE {
		return base, nil
	}
	epsilon, err := u.realNumeral("1/10000")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if zeroA, er := u.definitelyZero(a); er != nil {
		return ast.ASTHandle{}, er
	} else if zeroA {
		z, er := u.term.zero(b)
		if er != nil {
			return ast.ASTHandle{}, er
		}
		negative, er := u.manager.MkArithmeticLT(b, z)
		if er != nil {
			return ast.ASTHandle{}, er
		}
		one, er := u.realNumeral("1")
		if er != nil {
			return ast.ASTHandle{}, er
		}
		minusOne, er := u.realNumeral("-1")
		if er != nil {
			return ast.ASTHandle{}, er
		}
		direction, er := u.manager.MkIte(negative, one, minusOne)
		if er != nil {
			return ast.ASTHandle{}, er
		}
		offset, er := u.manager.MkArithmeticMul(epsilon, direction)
		if er != nil {
			return ast.ASTHandle{}, er
		}
		return u.manager.MkArithmeticAdd(base, offset)
	}
	if root.Multiplier > 0 {
		epsilon, err = u.manager.MkArithmeticNeg(epsilon)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return u.manager.MkArithmeticAdd(base, epsilon)
}

func (u *NonlinearArithmeticUtil) candidates(literal *NLLiteral) ([]nlCandidate, error) {
	a, b, c, err := u.coefficients(literal.Polynomial)
	if err != nil {
		return nil, err
	}
	zero, _, err := u.zeroOne(c)
	if err != nil {
		return nil, err
	}
	var result []nlCandidate
	bZero, err := u.definitelyZero(b)
	if err != nil {
		return nil, err
	}
	aZero, err := u.definitelyZero(a)
	if err != nil {
		return nil, err
	}
	if !bZero {
		minusC, er := u.manager.MkArithmeticNeg(c)
		if er != nil {
			return nil, er
		}
		root := NLRootForm{minusC, 0, zero, b}
		bNE, er := u.zeroComparison(b, NLComparisonNE)
		if er != nil {
			return nil, er
		}
		condition := bNE
		if !aZero {
			aEQ, er := u.zeroComparison(a, NLComparisonEQ)
			if er != nil {
				return nil, er
			}
			condition, er = u.and(aEQ, bNE)
			if er != nil {
				return nil, er
			}
		}
		definition, er := u.rootDefinition(root, literal.Comparison, zero, b)
		if er != nil {
			return nil, er
		}
		result = append(result, nlCandidate{root, condition, definition, a, b, c, literal.Comparison == NLComparisonLT || literal.Comparison == NLComparisonNE})
	}
	if !aZero {
		bb, er := u.manager.MkArithmeticMul(b, b)
		if er != nil {
			return nil, er
		}
		four, er := u.realNumeral("4")
		if er != nil {
			return nil, er
		}
		ac, er := u.manager.MkArithmeticMul(a, c)
		if er != nil {
			return nil, er
		}
		fourAC, er := u.manager.MkArithmeticMul(four, ac)
		if er != nil {
			return nil, er
		}
		discriminant, er := u.manager.MkArithmeticSub(bb, fourAC)
		if er != nil {
			return nil, er
		}
		two, er := u.realNumeral("2")
		if er != nil {
			return nil, er
		}
		denominator, er := u.manager.MkArithmeticMul(a, two)
		if er != nil {
			return nil, er
		}
		minusB, er := u.manager.MkArithmeticNeg(b)
		if er != nil {
			return nil, er
		}
		aNE, er := u.zeroComparison(a, NLComparisonNE)
		if er != nil {
			return nil, er
		}
		dGE, er := u.manager.MkArithmeticGE(discriminant, zero)
		if er != nil {
			return nil, er
		}
		condition, er := u.and(aNE, dGE)
		if er != nil {
			return nil, er
		}
		for _, sign := range []int{1, -1} {
			root := NLRootForm{minusB, sign, discriminant, denominator}
			definition, er := u.rootDefinition(root, literal.Comparison, a, b)
			if er != nil {
				return nil, er
			}
			result = append(result, nlCandidate{root, condition, definition, a, b, c, literal.Comparison == NLComparisonLT || literal.Comparison == NLComparisonNE})
		}
	}
	return result, nil
}

// singleQuadraticCandidates retains create_branch_l: the unique quadratic
// inequality contributes its vertex instead of both roots.
func (u *NonlinearArithmeticUtil) singleQuadraticCandidates(literal *NLLiteral, quadratic bool) ([]nlCandidate, error) {
	a, b, c, err := u.coefficients(literal.Polynomial)
	if err != nil {
		return nil, err
	}
	zero, _, err := u.zeroOne(c)
	if err != nil {
		return nil, err
	}
	comparison := literal.Comparison
	if quadratic {
		comparison = NLComparisonLE
	}
	var result []nlCandidate
	bZero, err := u.definitelyZero(b)
	if err != nil {
		return nil, err
	}
	aZero, err := u.definitelyZero(a)
	if err != nil {
		return nil, err
	}
	if !bZero {
		minusC, er := u.manager.MkArithmeticNeg(c)
		if er != nil {
			return nil, er
		}
		root := NLRootForm{minusC, 0, zero, b}
		bNE, er := u.zeroComparison(b, NLComparisonNE)
		if er != nil {
			return nil, er
		}
		condition := bNE
		if !aZero {
			aEQ, er := u.zeroComparison(a, NLComparisonEQ)
			if er != nil {
				return nil, er
			}
			condition, er = u.and(aEQ, bNE)
			if er != nil {
				return nil, er
			}
		}
		definition, er := u.rootDefinition(root, comparison, zero, b)
		if er != nil {
			return nil, er
		}
		strict := comparison == NLComparisonLT || comparison == NLComparisonNE
		result = append(result, nlCandidate{root, condition, definition, a, b, c, strict})
	}
	if quadratic && !aZero {
		two, er := u.realNumeral("2")
		if er != nil {
			return nil, er
		}
		denominator, er := u.manager.MkArithmeticMul(two, a)
		if er != nil {
			return nil, er
		}
		minusB, er := u.manager.MkArithmeticNeg(b)
		if er != nil {
			return nil, er
		}
		root := NLRootForm{minusB, 0, zero, denominator}
		condition, er := u.zeroComparison(a, NLComparisonNE)
		if er != nil {
			return nil, er
		}
		definition, er := u.rootDefinition(root, NLComparisonLE, a, b)
		if er != nil {
			return nil, er
		}
		result = append(result, nlCandidate{root, condition, definition, a, b, c, false})
	}
	return result, nil
}
