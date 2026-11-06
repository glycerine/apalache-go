package interp

// Source checkpoint: src/interp/iz3proof_itp.cpp rewrite-chain arithmetic
// normalization and coefficient recovery.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

var errSubtermNormals = errors.New("interpolation subterm normalization failed")

func (p *DirectProofInterpolator) positionAppend(position ast.ASTHandle, index int64) (ast.ASTHandle, error) {
	if position == p.topPosition {
		return p.positionAdd(index, position)
	}
	first, rest, err := p.positionIndex(position)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rest, err = p.positionAppend(rest, index)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.positionAdd(first, rest)
}

func (p *DirectProofInterpolator) extractRewrites(chain, position ast.ASTHandle) (ast.ASTHandle, error) {
	empty, err := p.base.manager.IsTrue(chain)
	if err != nil || empty {
		return chain, err
	}
	rest, last, err := p.chainParts(chain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rest, err = p.extractRewrites(rest, position)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	_, rewritePosition, _, _, err := p.rewriteFields(last)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	relation, difference, err := p.positionDifference(rewritePosition, position)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	switch relation {
	case -1:
		last, err = p.rewriteAtPosition(last, difference)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return p.chainCons(rest, last)
	case 1:
		left, right, e := p.rewriteEndpoints(last)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if left != right {
			return ast.ASTHandle{}, ProofError{}
		}
	}
	return rest, nil
}

func (p *DirectProofInterpolator) subtermNormals(first, second, chain ast.ASTHandle, normals *ast.ASTHandle, position ast.ASTHandle, seen map[ast.ASTHandle]struct{}, aProof, bProof *ast.ASTHandle) error {
	firstOp, err := p.base.manager.Operator(first)
	if err != nil {
		return err
	}
	secondOp, err := p.base.manager.Operator(second)
	if err != nil {
		return err
	}
	structural := firstOp == OperatorNot || firstOp == OperatorLE || firstOp == OperatorLT || firstOp == OperatorGE || firstOp == OperatorGT || firstOp == OperatorPlus || firstOp == OperatorTimes
	if structural {
		firstArgs, e := p.base.manager.Args(first)
		if e != nil {
			return e
		}
		secondArgs, e := p.base.manager.Args(second)
		if e != nil {
			return e
		}
		if secondOp != firstOp || len(secondArgs) != len(firstArgs) {
			return ProofError{}
		}
		for i := range firstArgs {
			childPosition, e := p.positionAppend(position, int64(i))
			if e != nil {
				return e
			}
			if e = p.subtermNormals(firstArgs[i], secondArgs[i], chain, normals, childPosition, seen, aProof, bProof); e != nil {
				return e
			}
		}
		return nil
	}
	secondSide, err := p.TermSide(second)
	if err != nil {
		return err
	}
	if secondSide == TermMixed {
		if _, found := seen[second]; found {
			return nil
		}
		seen[second] = struct{}{}
		subchain, e := p.extractRewrites(chain, position)
		if e != nil {
			return e
		}
		empty, e := p.base.manager.IsTrue(subchain)
		if e != nil {
			return e
		}
		if empty {
			return ProofError{}
		}
		subchain, e = p.reverseChain(subchain)
		if e != nil {
			return e
		}
		step, e := p.makeNormalStep(second, first, subchain)
		if e != nil {
			return e
		}
		truth, e := p.base.manager.Make(OperatorTrue)
		if e != nil {
			return e
		}
		addition, e := p.consNormal(step, truth)
		if e != nil {
			return e
		}
		*normals, e = p.mergeNormalChains(*normals, addition, aProof, bProof)
		return e
	}
	if first != second {
		return errSubtermNormals
	}
	return nil
}

func (p *DirectProofInterpolator) arithmeticRewriteCoefficient(inequality, position ast.ASTHandle) (coefficient, term, adjustedPosition ast.ASTHandle, err error) {
	one, err := p.base.manager.MakeIntText("1")
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if position == p.topPosition {
		return one, inequality, position, nil
	}
	index, rest, err := p.positionIndex(position)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	op, err := p.base.manager.Operator(inequality)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	coefficient = one
	switch op {
	case OperatorLE, OperatorLT:
		if index == 0 {
			coefficient, err = p.base.manager.MakeIntText("-1")
		}
	case OperatorGE, OperatorGT:
		if index != 0 {
			coefficient, err = p.base.manager.MakeIntText("-1")
		}
	case OperatorNot:
		coefficient, err = p.base.manager.MakeIntText("-1")
	case OperatorPlus:
	case OperatorTimes:
		coefficient, err = p.base.manager.Arg(inequality, 0)
	default:
		return one, inequality, p.topPosition, nil
	}
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if index < 0 {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, ProofError{}
	}
	argument, err := p.base.manager.Arg(inequality, uint32(index))
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	innerCoefficient, term, adjustedRest, err := p.arithmeticRewriteCoefficient(argument, rest)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	adjustedPosition, err = p.positionAdd(index, adjustedRest)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	value, numeral, err := p.base.manager.Numeral(coefficient)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if numeral && value.IsInt() && value.Num().Int64() == 1 {
		return innerCoefficient, term, adjustedPosition, nil
	}
	coefficient, err = p.base.manager.Make(OperatorTimes, coefficient, innerCoefficient)
	return coefficient, term, adjustedPosition, err
}

func (p *DirectProofInterpolator) rewritesToNormals(first, chain ast.ASTHandle, normals, aProof, bProof, aInequalities *ast.ASTHandle) (ast.ASTHandle, error) {
	empty, err := p.base.manager.IsTrue(chain)
	if err != nil || empty {
		return first, err
	}
	rest, last, err := p.chainParts(chain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	current, err := p.rewritesToNormals(first, rest, normals, aProof, bProof, aInequalities)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	_, rewritePosition, _, _, err := p.rewriteFields(last)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	coefficient, term1, arithmeticPosition, err := p.arithmeticRewriteCoefficient(current, rewritePosition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	_, replacement, err := p.rewriteEndpoints(last)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := p.substituteAtPosition(current, rewritePosition, replacement)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	_, relativePosition, err := p.positionDifference(arithmeticPosition, rewritePosition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	term2, err := p.substituteAtPosition(term1, relativePosition, replacement)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	term1Side, err := p.TermSide(term1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	term2Side, err := p.TermSide(term2)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if term1Side != TermMixed && term2Side != TermMixed {
		rewriteSide, _, _, _, e := p.rewriteFields(last)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if rewriteSide == TermA {
			zero, e := p.base.manager.MakeIntText("0")
			if e != nil {
				return ast.ASTHandle{}, e
			}
			difference, e := p.base.manager.Make(OperatorSub, term2, term1)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			inequality, e := p.base.manager.Make(OperatorLE, zero, difference)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			*aInequalities, e = p.base.manager.LinearCombination(*aInequalities, coefficient, inequality, false)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
	} else {
		truth, e := p.base.manager.Make(OperatorTrue)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		single, e := p.chainCons(truth, last)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		proof, e := p.extractRewrites(single, arithmeticPosition)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		step, e := p.fixNormal(term1, term2, proof)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		addition, e := p.consNormal(step, truth)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		*normals, e = p.mergeNormalChains(*normals, addition, aProof, bProof)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	return result, nil
}

func (p *DirectProofInterpolator) rewriteChainToNormalInequality(chain ast.ASTHandle, aProof, bProof *ast.ASTHandle) (ast.ASTHandle, error) {
	head, tail, err := p.headRewriteChain(chain, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	_, headRewrite, err := p.chainParts(head)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	headSide, _, _, _, err := p.rewriteFields(headRewrite)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	_, first, err := p.rewriteEndpoints(headRewrite)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	second, err := p.applyRewriteChain(first, tail)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	truth, err := p.base.manager.Make(OperatorTrue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	normals := truth
	zero, err := p.base.manager.MakeIntText("0")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	interpolation, err := p.base.manager.Make(OperatorLE, zero, zero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	savedA, savedB := *aProof, *bProof
	err = p.subtermNormals(first, second, tail, &normals, p.topPosition, make(map[ast.ASTHandle]struct{}), aProof, bProof)
	if errors.Is(err, errSubtermNormals) {
		*aProof, *bProof, normals = savedA, savedB, truth
		_, err = p.rewritesToNormals(first, tail, &normals, aProof, bProof, &interpolation)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if headSide == TermA {
		one, e := p.base.manager.MakeIntText("1")
		if e != nil {
			return ast.ASTHandle{}, e
		}
		interpolation, e = p.base.manager.LinearCombination(interpolation, one, first, false)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		proof, e := p.chainSideProof(TermB, head)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		proof, e = p.ownedSimplify(proof)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		*bProof, err = p.base.manager.MkAnd(*bProof, proof)
	} else {
		proof, e := p.chainSideProof(TermA, head)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		proof, e = p.ownedSimplify(proof)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		*aProof, err = p.base.manager.MkAnd(*aProof, proof)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	empty, err := p.base.manager.IsTrue(normals)
	if err != nil || empty {
		return interpolation, err
	}
	return p.makeNormal(interpolation, normals)
}
