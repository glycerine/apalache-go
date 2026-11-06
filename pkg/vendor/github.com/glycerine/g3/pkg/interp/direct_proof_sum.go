package interp

// Source checkpoint: src/interp/iz3proof_itp.cpp ordinary conditional
// inequality summation.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *DirectProofInterpolator) ownedIntegerDivision(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	ref, err := p.base.manager.MakeIDiv(left, right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := p.base.manager.ast.RefGet(ref)
	if err != nil {
		return ast.ASTHandle{}, errors.Join(err, p.base.manager.ast.DelRef(&ref))
	}
	p.owned = append(p.owned, ref)
	return result, nil
}

func (p *DirectProofInterpolator) ownedSimplify(term ast.ASTHandle) (ast.ASTHandle, error) {
	ref, err := p.base.manager.ReallySimplify(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := p.base.manager.ast.RefGet(ref)
	if err != nil {
		return ast.ASTHandle{}, errors.Join(err, p.base.manager.ast.DelRef(&ref))
	}
	p.owned = append(p.owned, ref)
	return result, nil
}

func (p *DirectProofInterpolator) isInequality(term ast.ASTHandle) (bool, error) {
	op, err := p.base.manager.Operator(term)
	if err != nil {
		return false, err
	}
	if op == OperatorNot {
		term, err = p.base.manager.Arg(term, 0)
		if err != nil {
			return false, err
		}
		op, err = p.base.manager.Operator(term)
		if err != nil {
			return false, err
		}
	}
	return op == OperatorLE || op == OperatorLT || op == OperatorGE || op == OperatorGT, nil
}

func (p *DirectProofInterpolator) destructConditionalInequality(term ast.ASTHandle, aProof, bProof *ast.ASTHandle) (ast.ASTHandle, error) {
	op, err := p.base.manager.Operator(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if op == OperatorAnd {
		condition, e := p.base.manager.Arg(term, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		*aProof, e = p.base.manager.MkAnd(*aProof, condition)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		term, e = p.base.manager.Arg(term, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		op, err = p.base.manager.Operator(term)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	if op == OperatorImplies {
		condition, e := p.base.manager.Arg(term, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		*bProof, e = p.base.manager.MkAnd(*bProof, condition)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return p.base.manager.Arg(term, 1)
	}
	return term, nil
}

func (p *DirectProofInterpolator) sumConditionalInequality(current, coefficient, addition ast.ASTHandle, aProof, bProof *ast.ASTHandle) (ast.ASTHandle, error) {
	op, err := p.base.manager.Operator(addition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if op == OperatorAnd || op == OperatorImplies {
		condition, e := p.base.manager.Arg(addition, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		body, e := p.base.manager.Arg(addition, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		current, e = p.sumConditionalInequality(current, coefficient, body, aProof, bProof)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if op == OperatorAnd {
			*aProof, e = p.base.manager.MkAnd(*aProof, condition)
		} else {
			*bProof, e = p.base.manager.MkAnd(*bProof, condition)
		}
		return current, e
	}
	additionOp, err := p.base.manager.Operator(addition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if additionOp == OperatorUninterpreted {
		declaration, e := p.base.manager.Symbol(addition)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if declaration == p.concat {
			addition, e = p.rewriteChainToNormalInequality(addition, aProof, bProof)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
	}
	currentNormal, err := p.isNormalWrapper(current)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	additionNormal, err := p.isNormalWrapper(addition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if currentNormal || additionNormal {
		return p.sumNormalInequalities(current, coefficient, addition, aProof, bProof)
	}
	inequality, err := p.isInequality(addition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !inequality {
		return ast.ASTHandle{}, ProofError{}
	}
	return p.base.manager.LinearCombination(current, coefficient, addition, false)
}

func (p *DirectProofInterpolator) simplifySum(args []ast.ASTHandle) (ast.ASTHandle, error) {
	if len(args) != 3 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	coefficient, numeral, err := p.base.manager.Numeral(args[1])
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !numeral || coefficient.Cmp(big.NewRat(1, 1)) != 0 {
		thirdOp, e := p.base.manager.Operator(args[2])
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if thirdOp == OperatorUninterpreted {
			declaration, e := p.base.manager.Symbol(args[2])
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if declaration == p.sum {
				one, e := p.base.manager.MakeIntText("1")
				if e != nil {
					return ast.ASTHandle{}, e
				}
				distributed, e := p.distributeSumCoefficient(args[1], args[2])
				if e != nil {
					return ast.ASTHandle{}, e
				}
				return p.makeSymbol(p.sum, args[0], one, distributed)
			}
		}
	}
	aProof, err := p.base.manager.Make(OperatorTrue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bProof := aProof
	inequality, err := p.destructConditionalInequality(args[0], &aProof, &bProof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	valid, err := p.isNormalInequality(inequality)
	if err != nil || !valid {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return ast.ASTHandle{}, ProofError{}
	}
	inequality, err = p.sumConditionalInequality(inequality, args[1], args[2], &aProof, &bProof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conditional, err := p.base.manager.MkImplies(bProof, inequality)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.base.manager.MkAnd(aProof, conditional)
}

func (p *DirectProofInterpolator) distributeSumCoefficient(coefficient, expression ast.ASTHandle) (ast.ASTHandle, error) {
	op, err := p.base.manager.Operator(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if op == OperatorUninterpreted {
		declaration, e := p.base.manager.Symbol(expression)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if declaration == p.sum {
			first, e := p.base.manager.Arg(expression, 0)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			innerCoefficient, e := p.base.manager.Arg(expression, 1)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			third, e := p.base.manager.Arg(expression, 2)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			first, e = p.distributeSumCoefficient(coefficient, first)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			thirdOp, e := p.base.manager.Operator(third)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if thirdOp == OperatorUninterpreted {
				thirdDeclaration, symbolErr := p.base.manager.Symbol(third)
				if symbolErr != nil {
					return ast.ASTHandle{}, symbolErr
				}
				if thirdDeclaration == p.sum {
					innerCoefficient, e = p.base.manager.Make(OperatorTimes, coefficient, innerCoefficient)
					if e != nil {
						return ast.ASTHandle{}, e
					}
					third, e = p.distributeSumCoefficient(innerCoefficient, third)
					if e != nil {
						return ast.ASTHandle{}, e
					}
					one, e := p.base.manager.MakeIntText("1")
					if e != nil {
						return ast.ASTHandle{}, e
					}
					return p.makeSymbol(p.sum, first, one, third)
				}
			}
			innerCoefficient, e = p.base.manager.Make(OperatorTimes, coefficient, innerCoefficient)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			return p.makeSymbol(p.sum, first, innerCoefficient, third)
		}
	}
	if op == OperatorLE {
		left, e := p.base.manager.Arg(expression, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		right, e := p.base.manager.Arg(expression, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		leftValue, leftNumeral, e := p.base.manager.Numeral(left)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		rightValue, rightNumeral, e := p.base.manager.Numeral(right)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if leftNumeral && rightNumeral && leftValue.Sign() == 0 && rightValue.Sign() == 0 {
			return expression, nil
		}
	}
	zero, err := p.base.manager.MakeIntText("0")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	base, err := p.base.manager.Make(OperatorLE, zero, zero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.makeSymbol(p.sum, base, coefficient, expression)
}

func (p *DirectProofInterpolator) divideInequality(inequality, divisor ast.ASTHandle) (ast.ASTHandle, error) {
	value, numeral, err := p.base.manager.Numeral(divisor)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if numeral && value.Cmp(big.NewRat(1, 1)) == 0 {
		return inequality, nil
	}
	op, err := p.base.manager.Operator(inequality)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if op == OperatorLT {
		left, e := p.base.manager.Arg(inequality, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		right, e := p.base.manager.Arg(inequality, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		one, e := p.base.manager.MakeIntText("1")
		if e != nil {
			return ast.ASTHandle{}, e
		}
		right, e = p.base.manager.Make(OperatorSub, right, one)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		inequality, e = p.base.manager.Make(OperatorLE, left, right)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		inequality, e = p.base.manager.SimplifyInequality(inequality)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		op = OperatorLE
	}
	if op != OperatorLE {
		return ast.ASTHandle{}, ProofError{}
	}
	left, err := p.base.manager.Arg(inequality, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := p.base.manager.Arg(inequality, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err = p.ownedIntegerDivision(left, divisor)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err = p.ownedIntegerDivision(right, divisor)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.base.manager.Make(op, left, right)
}

func (p *DirectProofInterpolator) simplifyRotatedSum(args []ast.ASTHandle) (ast.ASTHandle, error) {
	if len(args) != 2 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	placeholder, err := p.placeholder(args[0])
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zero, err := p.base.manager.MakeIntText("0")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	inequality, err := p.base.manager.Make(OperatorLE, zero, zero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	aProof, err := p.base.manager.Make(OperatorTrue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bProof := aProof
	var rotate func(ast.ASTHandle, ast.ASTHandle) (ast.ASTHandle, error)
	rotate = func(proof, current ast.ASTHandle) (ast.ASTHandle, error) {
		if proof == placeholder {
			return p.base.manager.SimplifyInequality(current)
		}
		op, e := p.base.manager.Operator(proof)
		if e != nil || op != OperatorUninterpreted {
			return ast.ASTHandle{}, ProofError{}
		}
		declaration, e := p.base.manager.Symbol(proof)
		if e != nil || declaration != p.sum {
			return ast.ASTHandle{}, ProofError{}
		}
		first, e := p.base.manager.Arg(proof, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		coefficient, e := p.base.manager.Arg(proof, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		third, e := p.base.manager.Arg(proof, 2)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if third == placeholder {
			one, e := p.base.manager.MakeIntText("1")
			if e != nil {
				return ast.ASTHandle{}, e
			}
			current, e = p.sumConditionalInequality(current, one, first, &aProof, &bProof)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			return p.divideInequality(current, coefficient)
		}
		current, e = p.sumConditionalInequality(current, coefficient, third, &aProof, &bProof)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return rotate(first, current)
	}
	inequality, err = rotate(args[1], inequality)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := p.base.manager.MkImplies(bProof, inequality)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.base.manager.MkAnd(aProof, result)
}

func (p *DirectProofInterpolator) roundInequality(inequality ast.ASTHandle) (ast.ASTHandle, error) {
	normal, err := p.isNormalWrapper(inequality)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if normal {
		body, chain, e := p.destructNormal(inequality)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		body, e = p.roundInequality(body)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return p.makeNormal(body, chain)
	}
	valid, err := p.isInequality(inequality)
	if err != nil || !valid {
		return ast.ASTHandle{}, ProofError{}
	}
	inequality, err = p.base.manager.SimplifyInequality(inequality)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	op, err := p.base.manager.Operator(inequality)
	if err != nil || op != OperatorLT {
		return inequality, err
	}
	left, err := p.base.manager.Arg(inequality, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := p.base.manager.Arg(inequality, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	one, err := p.base.manager.MakeIntText("1")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err = p.base.manager.Make(OperatorSub, right, one)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.base.manager.Make(OperatorLE, left, right)
}

func (p *DirectProofInterpolator) inequalityRight(inequality ast.ASTHandle) (ast.ASTHandle, error) {
	op, err := p.base.manager.Operator(inequality)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if op == OperatorImplies {
		body, e := p.base.manager.Arg(inequality, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return p.inequalityRight(body)
	}
	if op != OperatorLE && op != OperatorLT {
		return ast.ASTHandle{}, ProofError{}
	}
	return p.base.manager.Arg(inequality, 1)
}

func (p *DirectProofInterpolator) simplifyRotatedLeqToEq(placeholder, negativeEquality, proof ast.ASTHandle) (ast.ASTHandle, error) {
	first, err := p.base.manager.Arg(proof, 0)
	if err != nil || first != placeholder {
		return ast.ASTHandle{}, ProofError{}
	}
	equality, err := p.base.manager.Arg(negativeEquality, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	x, err := p.base.manager.Arg(equality, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	y, err := p.base.manager.Arg(equality, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	truth, err := p.base.manager.Make(OperatorTrue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	aProof1, bProof1 := truth, truth
	xLeqY, err := p.base.manager.Arg(proof, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	xLeqY, err = p.destructConditionalInequality(xLeqY, &aProof1, &bProof1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	yLeqX, err := p.base.manager.Arg(proof, 2)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	yLeqX, err = p.destructConditionalInequality(yLeqX, &aProof1, &bProof1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	xLeqY, err = p.roundInequality(xLeqY)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	yLeqX, err = p.roundInequality(yLeqX)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zero, err := p.base.manager.MakeIntText("0")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negativeOne, err := p.base.manager.MakeIntText("-1")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	one, err := p.base.manager.MakeIntText("1")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	firstCondition, err := p.base.manager.Make(OperatorLE, zero, zero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	firstCondition, err = p.sumConditionalInequality(firstCondition, negativeOne, xLeqY, &aProof1, &bProof1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	firstCondition, err = p.sumConditionalInequality(firstCondition, negativeOne, yLeqX, &aProof1, &bProof1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	firstCondition, err = p.ownedSimplify(firstCondition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	firstCondition, err = p.base.manager.MkAnd(bProof1, firstCondition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	firstCondition, err = p.base.manager.MkImplies(aProof1, firstCondition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	aProof2, bProof2 := truth, truth
	secondCondition, err := p.base.manager.Make(OperatorLE, zero, zero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	secondCondition, err = p.sumConditionalInequality(secondCondition, one, xLeqY, &aProof2, &bProof2)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	secondCondition, err = p.sumConditionalInequality(secondCondition, one, yLeqX, &aProof2, &bProof2)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	secondCondition, err = p.ownedSimplify(secondCondition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// Preserve the selected source's use of the first condition pair here.
	secondCondition, err = p.base.manager.MkAnd(aProof1, secondCondition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	secondCondition, err = p.base.manager.MkImplies(bProof1, secondCondition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	xRight, err := p.inequalityRight(xLeqY)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	yRight, err := p.inequalityRight(yLeqX)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	makePair := func(firstSide TermSide, firstPosition ast.ASTHandle, firstLeft, firstRight, secondLeft, secondRight ast.ASTHandle) (ast.ASTHandle, error) {
		firstEquality, e := p.base.manager.Make(OperatorEqual, firstLeft, firstRight)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		firstRewrite, e := p.makeRewrite(firstSide, firstPosition, firstCondition, firstEquality)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		secondEquality, e := p.base.manager.Make(OperatorEqual, secondLeft, secondRight)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		secondSide := TermA
		if firstSide == TermA {
			secondSide = TermB
		}
		secondRewrite, e := p.makeRewrite(secondSide, p.topPosition, secondCondition, secondEquality)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		chain, e := p.chainCons(truth, firstRewrite)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return p.chainCons(chain, secondRewrite)
	}
	yValue, yNumeral, err := p.base.manager.Numeral(y)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	xOp, err := p.base.manager.Operator(x)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if yNumeral && yValue.Sign() == 0 && xOp == OperatorPlus {
		xArgs, e := p.base.manager.Args(x)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if len(xArgs) == 2 {
			for index := 0; index < 2; index++ {
				side, e := p.TermSide(xArgs[index])
				if e != nil {
					return ast.ASTHandle{}, e
				}
				if side == TermA {
					iteration, e := p.base.manager.Make(OperatorPlus, xArgs[index], xRight)
					if e != nil {
						return ast.ASTHandle{}, e
					}
					iteration, e = p.ownedSimplify(iteration)
					if e != nil {
						return ast.ASTHandle{}, e
					}
					position, e := p.positionAdd(int64(index), p.topPosition)
					if e != nil {
						return ast.ASTHandle{}, e
					}
					updated := []ast.ASTHandle{xArgs[0], xArgs[1]}
					updated[index] = iteration
					whole, e := p.base.manager.Make(OperatorPlus, updated...)
					if e != nil {
						return ast.ASTHandle{}, e
					}
					return makePair(TermA, position, xArgs[index], iteration, whole, y)
				}
			}
		}
	}
	xSide, err := p.TermSide(x)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if xSide == TermA {
		iteration, e := p.base.manager.Make(OperatorPlus, x, xRight)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		iteration, e = p.ownedSimplify(iteration)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return makePair(TermA, p.topPosition, x, iteration, iteration, y)
	}
	ySide, err := p.TermSide(y)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if ySide == TermA {
		iteration, e := p.base.manager.Make(OperatorPlus, y, yRight)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		iteration, e = p.ownedSimplify(iteration)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		// This source branch emits the B rewrite first and the A rewrite second.
		firstEquality, e := p.base.manager.Make(OperatorEqual, x, iteration)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		firstRewrite, e := p.makeRewrite(TermB, p.topPosition, secondCondition, firstEquality)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		secondEquality, e := p.base.manager.Make(OperatorEqual, iteration, y)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		secondRewrite, e := p.makeRewrite(TermA, p.topPosition, firstCondition, secondEquality)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		chain, e := p.chainCons(truth, firstRewrite)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return p.chainCons(chain, secondRewrite)
	}
	return ast.ASTHandle{}, ProofError{}
}
