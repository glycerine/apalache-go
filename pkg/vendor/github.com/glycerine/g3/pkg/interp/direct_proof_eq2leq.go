package interp

// Source checkpoint: src/interp/iz3proof_itp.cpp equality-to-inequality
// proof rotation and mixed arithmetic movers.

import "github.com/glycerine/g3/pkg/ast"

func (p *DirectProofInterpolator) hasMixedSummands(term ast.ASTHandle) (bool, error) {
	op, err := p.base.manager.Operator(term)
	if err != nil {
		return false, err
	}
	if op == OperatorPlus {
		args, e := p.base.manager.Args(term)
		if e != nil {
			return false, e
		}
		for _, argument := range args {
			mixed, e := p.hasMixedSummands(argument)
			if e != nil || mixed {
				return mixed, e
			}
		}
		return false, nil
	}
	side, err := p.TermSide(term)
	return side == TermMixed, err
}

func (p *DirectProofInterpolator) rightRewriteMovers(chain, right ast.ASTHandle) (head, tail, middle ast.ASTHandle, err error) {
	empty, err := p.base.manager.IsTrue(chain)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	mixed, err := p.hasMixedSummands(right)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if empty || !mixed {
		truth, e := p.base.manager.Make(OperatorTrue)
		return chain, truth, right, e
	}
	rest, last, err := p.chainParts(chain)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	_, position, _, _, err := p.rewriteFields(last)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	left, _, err := p.rewriteEndpoints(last)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	previous, err := p.substituteAtPosition(right, position, left)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	head, tail, middle, err = p.rightRewriteMovers(rest, previous)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	tail, err = p.chainCons(tail, last)
	return head, tail, middle, err
}

func (p *DirectProofInterpolator) leftRewriteMovers(chain, left ast.ASTHandle) (head, tail, middle ast.ASTHandle, found bool, err error) {
	empty, err := p.base.manager.IsTrue(chain)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if empty {
		mixed, e := p.hasMixedSummands(left)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		if !mixed {
			truth, e := p.base.manager.Make(OperatorTrue)
			return chain, truth, left, true, e
		}
		return ast.ASTHandle{}, ast.ASTHandle{}, left, false, nil
	}
	rest, last, err := p.chainParts(chain)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	head, tail, middle, found, err = p.leftRewriteMovers(rest, left)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if !found {
		_, position, _, _, e := p.rewriteFields(last)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		_, right, e := p.rewriteEndpoints(last)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		middle, e = p.substituteAtPosition(middle, position, right)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		side, e := p.TermSide(middle)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		if side != TermMixed {
			truth, e := p.base.manager.Make(OperatorTrue)
			return chain, truth, middle, true, e
		}
		return ast.ASTHandle{}, ast.ASTHandle{}, middle, false, nil
	}
	tail, err = p.chainCons(tail, last)
	return head, tail, middle, true, err
}

func (p *DirectProofInterpolator) rewritesFromTo(chain, left, right ast.ASTHandle) (bool, error) {
	empty, err := p.base.manager.IsTrue(chain)
	if err != nil || empty {
		return empty && left == right, err
	}
	rest, last, err := p.chainParts(chain)
	if err != nil {
		return false, err
	}
	_, position, _, _, err := p.rewriteFields(last)
	if err != nil {
		return false, err
	}
	rewriteLeft, _, err := p.rewriteEndpoints(last)
	if err != nil {
		return false, err
	}
	middle, err := p.substituteAtPosition(right, position, rewriteLeft)
	if err != nil {
		return false, err
	}
	return p.rewritesFromTo(rest, left, middle)
}

func (p *DirectProofInterpolator) chainInequalities(comparison Operator, side TermSide, chain, left, right ast.ASTHandle) (ast.ASTHandle, error) {
	empty, err := p.base.manager.IsTrue(chain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zero, err := p.base.manager.MakeIntText("0")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if empty {
		if left != right {
			return ast.ASTHandle{}, ProofError{}
		}
		return p.base.manager.Make(OperatorLE, zero, zero)
	}
	rest, last, err := p.chainParts(chain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rewriteSide, position, _, _, err := p.rewriteFields(last)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rewriteLeft, _, err := p.rewriteEndpoints(last)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	middle, err := p.substituteAtPosition(right, position, rewriteLeft)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	condition, err := p.chainInequalities(comparison, side, rest, left, middle)
	if err != nil || rewriteSide != side {
		return condition, err
	}
	var difference ast.ASTHandle
	if comparison == OperatorLE {
		difference, err = p.base.manager.Make(OperatorSub, right, middle)
	} else {
		difference, err = p.base.manager.Make(OperatorSub, middle, right)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	difference, err = p.ownedSimplify(difference)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	addition, err := p.base.manager.Make(OperatorLE, zero, difference)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conditionTrue, err := p.base.manager.IsTrue(condition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if conditionTrue {
		return addition, nil
	}
	one, err := p.base.manager.MakeIntText("1")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	condition, err = p.base.manager.LinearCombination(condition, one, addition, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.base.manager.SimplifyInequality(condition)
}

func (p *DirectProofInterpolator) equalityEndpointsFromInequality(inequality ast.ASTHandle) (left, right ast.ASTHandle, err error) {
	op, err := p.base.manager.Operator(inequality)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	left, err = p.base.manager.Arg(inequality, 0)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	right, err = p.base.manager.Arg(inequality, 1)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	leftOp, err := p.base.manager.Operator(left)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	rightValue, rightNumeral, err := p.base.manager.Numeral(right)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if leftOp == OperatorPlus && rightNumeral && rightValue.Sign() == 0 {
		args, e := p.base.manager.Args(left)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		if len(args) == 2 {
			candidateOp, e := p.base.manager.Operator(args[1])
			if e != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, e
			}
			if candidateOp == OperatorTimes {
				candidate, e := p.base.manager.Arg(args[1], 1)
				return args[0], candidate, e
			}
		}
	}
	if op == OperatorLE || op == OperatorGE {
		return left, right, nil
	}
	return ast.ASTHandle{}, ast.ASTHandle{}, ProofError{}
}

func (p *DirectProofInterpolator) unmixedEqToInequality(left, right ast.ASTHandle, comparison Operator, chain ast.ASTHandle, condition *ast.ASTHandle) (ast.ASTHandle, error) {
	inequalities, err := p.chainInequalities(comparison, TermA, chain, left, right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	aConditions, err := p.chainSideConjunction(TermA, chain, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	*condition, err = p.base.manager.MkAnd(*condition, aConditions)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bConditions, err := p.chainSideConjunction(TermB, chain, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bConditions, err = p.ownedSimplify(bConditions)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	combined, err := p.base.manager.MkImplies(*condition, inequalities)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bTrue, err := p.base.manager.IsTrue(bConditions)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ineqOp, err := p.base.manager.Operator(inequalities)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if bTrue && ineqOp != OperatorAnd {
		return combined, nil
	}
	if ineqOp != OperatorAnd {
		return p.base.manager.MkAnd(bConditions, combined)
	}
	return ast.ASTHandle{}, ProofError{}
}

func (p *DirectProofInterpolator) fixNormal(left, right, proof ast.ASTHandle) (ast.ASTHandle, error) {
	leftSide, err := p.TermSide(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightSide, err := p.TermSide(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	leftID, err := p.base.manager.ASTID(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightID, err := p.base.manager.ASTID(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if leftSide == TermMixed && (rightSide != TermMixed || leftID < rightID) {
		return p.makeNormalStep(left, right, proof)
	}
	if rightSide == TermMixed && (leftSide != TermMixed || rightID < leftID) {
		proof, err = p.reverseChain(proof)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return p.makeNormalStep(right, left, proof)
	}
	return ast.ASTHandle{}, ProofError{}
}

func (p *DirectProofInterpolator) addMixedEqToInequality(left, right, chain, interpolation ast.ASTHandle) (ast.ASTHandle, error) {
	empty, err := p.base.manager.IsTrue(chain)
	if err != nil || empty {
		return interpolation, err
	}
	zero, err := p.base.manager.MakeIntText("0")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	one, err := p.base.manager.MakeIntText("1")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	inequality, err := p.base.manager.Make(OperatorLE, zero, zero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	step, err := p.fixNormal(left, right, chain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	truth, err := p.base.manager.Make(OperatorTrue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	normalChain, err := p.consNormal(step, truth)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	normal, err := p.makeNormal(inequality, normalChain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.simplifySum([]ast.ASTHandle{interpolation, one, normal})
}

func (p *DirectProofInterpolator) simplifyRotatedEqToLeq(placeholder, negativeInequality, proof ast.ASTHandle) (ast.ASTHandle, error) {
	second, err := p.base.manager.Arg(proof, 1)
	if err != nil || second != placeholder {
		return ast.ASTHandle{}, ProofError{}
	}
	truth, err := p.base.manager.Make(OperatorTrue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	condition := truth
	chain, err := p.base.manager.Arg(proof, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	chain, err = p.separateRewriteCondition(chain, &condition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	equivalence, err := p.isEquivalenceChain(chain)
	if err != nil || !equivalence {
		return ast.ASTHandle{}, ProofError{}
	}
	inequality, err := p.base.manager.Arg(negativeInequality, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	comparison, err := p.base.manager.Operator(inequality)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, right, err := p.equalityEndpointsFromInequality(inequality)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	matches, err := p.rewritesFromTo(chain, left, right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !matches {
		left, err = p.base.manager.Arg(inequality, 0)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		right, err = p.base.manager.Arg(inequality, 1)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	leftSide, err := p.TermSide(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightSide, err := p.TermSide(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if leftSide != TermMixed && rightSide != TermMixed {
		return p.unmixedEqToInequality(left, right, comparison, chain, &condition)
	}
	leftChain, middle, leftTerm, found, err := p.leftRewriteMovers(chain, left)
	if err != nil || !found {
		return ast.ASTHandle{}, ProofError{}
	}
	middle, rightChain, rightTerm, err := p.rightRewriteMovers(middle, right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	interpolation, err := p.unmixedEqToInequality(leftTerm, rightTerm, comparison, middle, &condition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	interpolation, err = p.addMixedEqToInequality(left, leftTerm, leftChain, interpolation)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.addMixedEqToInequality(rightTerm, right, rightChain, interpolation)
}
