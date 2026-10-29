package interp

// Source checkpoint: src/interp/iz3proof_itp.cpp arithmetic normalization
// proof vocabulary and basic chain accessors.

import "github.com/glycerine/g3/pkg/ast"

func (p *DirectProofInterpolator) makeNormalStep(left, right, proof ast.ASTHandle) (ast.ASTHandle, error) {
	equality, err := p.makeEquivalence(left, right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.makeSymbol(p.normalStep, equality, proof)
}

func (p *DirectProofInterpolator) consNormal(first, rest ast.ASTHandle) (ast.ASTHandle, error) {
	return p.makeSymbol(p.normalChain, first, rest)
}

func (p *DirectProofInterpolator) normalChainParts(chain ast.ASTHandle) (first, rest ast.ASTHandle, err error) {
	op, err := p.base.manager.Operator(chain)
	if err != nil || op != OperatorUninterpreted {
		return ast.ASTHandle{}, ast.ASTHandle{}, ProofError{}
	}
	declaration, err := p.base.manager.Symbol(chain)
	if err != nil || declaration != p.normalChain {
		return ast.ASTHandle{}, ast.ASTHandle{}, ProofError{}
	}
	first, err = p.base.manager.Arg(chain, 0)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	rest, err = p.base.manager.Arg(chain, 1)
	return first, rest, err
}

func (p *DirectProofInterpolator) normalStepParts(step ast.ASTHandle) (left, right, proof ast.ASTHandle, err error) {
	op, err := p.base.manager.Operator(step)
	if err != nil || op != OperatorUninterpreted {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, ProofError{}
	}
	declaration, err := p.base.manager.Symbol(step)
	if err != nil || declaration != p.normalStep {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, ProofError{}
	}
	equality, err := p.base.manager.Arg(step, 0)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	left, err = p.base.manager.Arg(equality, 0)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	right, err = p.base.manager.Arg(equality, 1)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	proof, err = p.base.manager.Arg(step, 1)
	return left, right, proof, err
}

func (p *DirectProofInterpolator) makeNormal(inequality, chain ast.ASTHandle) (ast.ASTHandle, error) {
	valid, err := p.isInequality(inequality)
	if err != nil || !valid {
		return ast.ASTHandle{}, ProofError{}
	}
	return p.makeSymbol(p.normal, inequality, chain)
}

func (p *DirectProofInterpolator) destructNormal(proof ast.ASTHandle) (inequality, chain ast.ASTHandle, err error) {
	op, err := p.base.manager.Operator(proof)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if op == OperatorUninterpreted {
		declaration, e := p.base.manager.Symbol(proof)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		if declaration == p.normal {
			inequality, e = p.base.manager.Arg(proof, 0)
			if e != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, e
			}
			chain, e = p.base.manager.Arg(proof, 1)
			return inequality, chain, e
		}
	}
	chain, err = p.base.manager.Make(OperatorTrue)
	return proof, chain, err
}

func (p *DirectProofInterpolator) isNormalInequality(proof ast.ASTHandle) (bool, error) {
	op, err := p.base.manager.Operator(proof)
	if err != nil {
		return false, err
	}
	if op == OperatorUninterpreted {
		declaration, e := p.base.manager.Symbol(proof)
		if e != nil {
			return false, e
		}
		if declaration == p.normal {
			inequality, e := p.base.manager.Arg(proof, 0)
			if e != nil {
				return false, e
			}
			return p.isInequality(inequality)
		}
	}
	return p.isInequality(proof)
}

func (p *DirectProofInterpolator) isNormalWrapper(proof ast.ASTHandle) (bool, error) {
	op, err := p.base.manager.Operator(proof)
	if err != nil || op != OperatorUninterpreted {
		return false, err
	}
	declaration, err := p.base.manager.Symbol(proof)
	return declaration == p.normal, err
}

func (p *DirectProofInterpolator) sumNormalInequalities(first, coefficient, second ast.ASTHandle, aProof, bProof *ast.ASTHandle) (ast.ASTHandle, error) {
	firstInequality, firstChain, err := p.destructNormal(first)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	secondInequality, secondChain, err := p.destructNormal(second)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	truth, err := p.base.manager.Make(OperatorTrue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	dummyA, dummyB := truth, truth
	firstInequality, err = p.sumConditionalInequality(firstInequality, coefficient, secondInequality, &dummyA, &dummyB)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	chain, err := p.mergeNormalChains(firstChain, secondChain, aProof, bProof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	empty, err := p.base.manager.IsTrue(chain)
	if err != nil || empty {
		return firstInequality, err
	}
	return p.makeNormal(firstInequality, chain)
}

func (p *DirectProofInterpolator) normalizeProofTerm(term ast.ASTHandle) (ast.ASTHandle, error) {
	truth, err := p.base.manager.Make(OperatorTrue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	aProof, bProof := truth, truth
	current := term
	op, err := p.base.manager.Operator(current)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if op == OperatorAnd {
		aProof, err = p.base.manager.Arg(current, 0)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		current, err = p.base.manager.Arg(current, 1)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		op, err = p.base.manager.Operator(current)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	if op == OperatorImplies {
		bProof, err = p.base.manager.Arg(current, 0)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		current, err = p.base.manager.Arg(current, 1)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	normal, err := p.isNormalWrapper(current)
	if err != nil || !normal {
		return term, err
	}
	inequality, chain, err := p.destructNormal(current)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	replacements := make(map[ast.ASTHandle]ast.ASTHandle)
	for {
		empty, e := p.base.manager.IsTrue(chain)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if empty {
			break
		}
		step, rest, e := p.normalChainParts(chain)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		left, right, _, e := p.normalStepParts(step)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		replacements[left] = right
		chain = rest
	}
	inequality, err = p.base.manager.Substitute(replacements, inequality)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := p.base.manager.MkImplies(bProof, inequality)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.base.manager.MkAnd(aProof, result)
}

func (p *DirectProofInterpolator) mergeNormalChains(first, second ast.ASTHandle, aProof, bProof *ast.ASTHandle) (ast.ASTHandle, error) {
	transitive := make(map[ast.ASTHandle]ast.ASTHandle)
	result, err := p.mergeNormalChainsRecursive(first, second, transitive, aProof, bProof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.transitiveNormalChain(result, transitive)
}

func (p *DirectProofInterpolator) mergeNormalChainsRecursive(first, second ast.ASTHandle, transitive map[ast.ASTHandle]ast.ASTHandle, aProof, bProof *ast.ASTHandle) (ast.ASTHandle, error) {
	firstEmpty, err := p.base.manager.IsTrue(first)
	if err != nil || firstEmpty {
		return second, err
	}
	secondEmpty, err := p.base.manager.IsTrue(second)
	if err != nil || secondEmpty {
		return first, err
	}
	firstStep, firstRest, err := p.normalChainParts(first)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	secondStep, secondRest, err := p.normalChainParts(second)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left1, right1, proof1, err := p.normalStepParts(firstStep)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left2, right2, proof2, err := p.normalStepParts(secondStep)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	id1, err := p.base.manager.ASTID(left1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	id2, err := p.base.manager.ASTID(left2)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if id1 < id2 {
		rest, e := p.mergeNormalChainsRecursive(firstRest, second, transitive, aProof, bProof)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return p.consNormal(firstStep, rest)
	}
	if id2 < id1 {
		rest, e := p.mergeNormalChainsRecursive(first, secondRest, transitive, aProof, bProof)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return p.consNormal(secondStep, rest)
	}
	side1, err := p.TermSide(right1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	side2, err := p.TermSide(right2)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightID1, err := p.base.manager.ASTID(right1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightID2, err := p.base.manager.ASTID(right2)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var mergedStep ast.ASTHandle
	if side1 == TermMixed && (side2 != TermMixed || rightID2 > rightID1) {
		reversed, e := p.reverseChain(proof1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		combined, e := p.concatRewriteChains(reversed, proof2)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		mergedStep = secondStep
		transitive[right1], e = p.makeNormalStep(right1, right2, combined)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	} else if side2 == TermMixed && (side1 != TermMixed || rightID1 > rightID2) {
		return p.mergeNormalChainsRecursive(second, first, transitive, aProof, bProof)
	} else if side1 == TermA && side2 == TermB {
		reversed, e := p.reverseChain(proof1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		combined, e := p.concatRewriteChains(reversed, proof2)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		aRewrites, bRewrites, e := p.dropRewrites(TermB, combined)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		condition, e := p.chainSideProof(TermB, aRewrites)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		*bProof, e = p.base.manager.MkAnd(*bProof, condition)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		condition, e = p.chainSideProof(TermA, bRewrites)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		*aProof, e = p.base.manager.MkAnd(*aProof, condition)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		representative, e := p.applyRewriteChain(right1, aRewrites)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		combined, e = p.concatRewriteChains(proof1, aRewrites)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		mergedStep, e = p.makeNormalStep(left1, representative, combined)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		aNormal, e := p.makeNormalStep(right1, representative, aRewrites)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		rest, e := p.mergeNormalChainsRecursive(firstRest, secondRest, transitive, aProof, bProof)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		result, e := p.consNormal(mergedStep, rest)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		truth, e := p.base.manager.Make(OperatorTrue)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		aChain, e := p.consNormal(aNormal, truth)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return p.mergeNormalChainsRecursive(result, aChain, transitive, aProof, bProof)
	} else if side1 == TermB && side2 == TermA {
		return p.mergeNormalChainsRecursive(second, first, transitive, aProof, bProof)
	} else {
		reversed, e := p.reverseChain(proof1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		combined, e := p.concatRewriteChains(reversed, proof2)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if side1 == TermA {
			condition, e := p.chainSideProof(TermB, combined)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			*bProof, e = p.base.manager.MkAnd(*bProof, condition)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		} else {
			condition, e := p.chainSideProof(TermA, combined)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			*aProof, e = p.base.manager.MkAnd(*aProof, condition)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		mergedStep = firstStep
	}
	rest, err := p.mergeNormalChainsRecursive(firstRest, secondRest, transitive, aProof, bProof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.consNormal(mergedStep, rest)
}

func (p *DirectProofInterpolator) transitiveNormalChain(chain ast.ASTHandle, transitive map[ast.ASTHandle]ast.ASTHandle) (ast.ASTHandle, error) {
	empty, err := p.base.manager.IsTrue(chain)
	if err != nil || empty {
		return chain, err
	}
	step, rest, err := p.normalChainParts(chain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rest, err = p.transitiveNormalChain(rest, transitive)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, right, proof, err := p.normalStepParts(step)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	leftSide, err := p.TermSide(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if next, found := transitive[right]; found && leftSide == TermMixed {
		_, nextRight, nextProof, e := p.normalStepParts(next)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		proof, e = p.concatRewriteChains(proof, nextProof)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		step, e = p.makeNormalStep(left, nextRight, proof)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	if leftSide == TermMixed {
		transitive[left] = step
	}
	return p.consNormal(step, rest)
}
