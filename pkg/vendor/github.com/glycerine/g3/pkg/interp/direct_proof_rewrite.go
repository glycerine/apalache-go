package interp

// Source checkpoint: src/interp/iz3proof_itp.cpp local rewrite construction
// and trivial mixed-rule interpolation.

import (
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *DirectProofInterpolator) isEquivalence(term ast.ASTHandle) (bool, error) {
	op, err := p.base.manager.Operator(term)
	return op == OperatorEqual || op == OperatorIff, err
}

func (p *DirectProofInterpolator) makeRewrite(side TermSide, position, condition, equality ast.ASTHandle) (ast.ASTHandle, error) {
	equivalence, err := p.isEquivalence(equality)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !equivalence {
		return ast.ASTHandle{}, ProofError{}
	}
	var declaration ast.ASTHandle
	switch side {
	case TermA:
		declaration = p.rewriteA
	case TermB:
		declaration = p.rewriteB
	default:
		return ast.ASTHandle{}, ProofError{}
	}
	return p.makeSymbol(declaration, position, condition, equality)
}

func (p *DirectProofInterpolator) chainCons(chain, element ast.ASTHandle) (ast.ASTHandle, error) {
	return p.makeSymbol(p.concat, chain, element)
}

func (p *DirectProofInterpolator) chainParts(chain ast.ASTHandle) (rest, last ast.ASTHandle, err error) {
	op, err := p.base.manager.Operator(chain)
	if err != nil || op != OperatorUninterpreted {
		return ast.ASTHandle{}, ast.ASTHandle{}, ProofError{}
	}
	declaration, err := p.base.manager.Symbol(chain)
	if err != nil || declaration != p.concat {
		return ast.ASTHandle{}, ast.ASTHandle{}, ProofError{}
	}
	rest, err = p.base.manager.Arg(chain, 0)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	last, err = p.base.manager.Arg(chain, 1)
	return rest, last, err
}

func (p *DirectProofInterpolator) reverseRewrite(rewrite ast.ASTHandle) (ast.ASTHandle, error) {
	declaration, err := p.base.manager.Symbol(rewrite)
	if err != nil || (declaration != p.rewriteA && declaration != p.rewriteB) {
		return ast.ASTHandle{}, ProofError{}
	}
	position, err := p.base.manager.Arg(rewrite, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	condition, err := p.base.manager.Arg(rewrite, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	equality, err := p.base.manager.Arg(rewrite, 2)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	op, err := p.base.manager.Operator(equality)
	if err != nil || (op != OperatorEqual && op != OperatorIff) {
		return ast.ASTHandle{}, ProofError{}
	}
	left, err := p.base.manager.Arg(equality, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := p.base.manager.Arg(equality, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	equality, err = p.base.manager.Make(op, right, left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.makeSymbol(declaration, position, condition, equality)
}

func (p *DirectProofInterpolator) reverseChain(chain ast.ASTHandle) (ast.ASTHandle, error) {
	result, err := p.base.manager.Make(OperatorTrue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for {
		empty, e := p.base.manager.IsTrue(chain)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if empty {
			return result, nil
		}
		rest, last, e := p.chainParts(chain)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		last, e = p.reverseRewrite(last)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		result, e = p.chainCons(result, last)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		chain = rest
	}
}

func (p *DirectProofInterpolator) rewriteFields(rewrite ast.ASTHandle) (side TermSide, position, condition, equality ast.ASTHandle, err error) {
	declaration, err := p.base.manager.Symbol(rewrite)
	if err != nil {
		return TermMixed, ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	switch declaration {
	case p.rewriteA:
		side = TermA
	case p.rewriteB:
		side = TermB
	default:
		return TermMixed, ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, ProofError{}
	}
	position, err = p.base.manager.Arg(rewrite, 0)
	if err != nil {
		return TermMixed, ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	condition, err = p.base.manager.Arg(rewrite, 1)
	if err != nil {
		return TermMixed, ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	equality, err = p.base.manager.Arg(rewrite, 2)
	return side, position, condition, equality, err
}

func (p *DirectProofInterpolator) positionAdd(index int64, position ast.ASTHandle) (ast.ASTHandle, error) {
	indexTerm, err := p.base.manager.MakeIntText(strconv.FormatInt(index, 10))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.makeSymbol(p.addPosition, indexTerm, position)
}

func (p *DirectProofInterpolator) positionIndex(position ast.ASTHandle) (int64, ast.ASTHandle, error) {
	declaration, err := p.base.manager.Symbol(position)
	if err != nil || declaration != p.addPosition {
		return 0, ast.ASTHandle{}, ProofError{}
	}
	indexTerm, err := p.base.manager.Arg(position, 0)
	if err != nil {
		return 0, ast.ASTHandle{}, err
	}
	index, matched, err := p.base.manager.Numeral(indexTerm)
	if err != nil || !matched || !index.IsInt() || !index.Num().IsInt64() {
		return 0, ast.ASTHandle{}, ProofError{}
	}
	rest, err := p.base.manager.Arg(position, 1)
	return index.Num().Int64(), rest, err
}

func (p *DirectProofInterpolator) positionDifference(first, second ast.ASTHandle) (int, ast.ASTHandle, error) {
	if first == p.topPosition {
		if second == p.topPosition {
			return 0, p.topPosition, nil
		}
		return 1, second, nil
	}
	if second == p.topPosition {
		return -1, first, nil
	}
	firstIndex, firstRest, err := p.positionIndex(first)
	if err != nil {
		return 0, ast.ASTHandle{}, err
	}
	secondIndex, secondRest, err := p.positionIndex(second)
	if err != nil {
		return 0, ast.ASTHandle{}, err
	}
	if firstIndex != secondIndex {
		return 2, ast.ASTHandle{}, nil
	}
	return p.positionDifference(firstRest, secondRest)
}

func (p *DirectProofInterpolator) substituteAtPosition(term, position, replacement ast.ASTHandle) (ast.ASTHandle, error) {
	if position == p.topPosition {
		return replacement, nil
	}
	index, rest, err := p.positionIndex(position)
	if err != nil || index < 0 {
		return ast.ASTHandle{}, ProofError{}
	}
	args, err := p.base.manager.Args(term)
	if err != nil || index >= int64(len(args)) {
		return ast.ASTHandle{}, ProofError{}
	}
	args[index], err = p.substituteAtPosition(args[index], rest, replacement)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.base.manager.Clone(term, args)
}

func (p *DirectProofInterpolator) updateRewrite(rewrite, position, replacement, extraCondition ast.ASTHandle, updateRight bool) (ast.ASTHandle, error) {
	side, rewritePosition, condition, equality, err := p.rewriteFields(rewrite)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	op, err := p.base.manager.Operator(equality)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err := p.base.manager.Arg(equality, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := p.base.manager.Arg(equality, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if updateRight {
		right, err = p.substituteAtPosition(right, position, replacement)
	} else {
		left, err = p.substituteAtPosition(left, position, replacement)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	condition, err = p.base.manager.MkAnd(condition, extraCondition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	equality, err = p.base.manager.Make(op, left, right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.makeRewrite(side, rewritePosition, condition, equality)
}

func (p *DirectProofInterpolator) rewriteEndpoints(rewrite ast.ASTHandle) (left, right ast.ASTHandle, err error) {
	_, _, _, equality, err := p.rewriteFields(rewrite)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	left, err = p.base.manager.Arg(equality, 0)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	right, err = p.base.manager.Arg(equality, 1)
	return left, right, err
}

func (p *DirectProofInterpolator) termCommon(term ast.ASTHandle) (bool, error) {
	scope, err := p.base.ASTScope(term)
	if err != nil {
		return false, err
	}
	intersects, err := p.base.scopes.RangesIntersect(scope, p.scope)
	if err != nil {
		return false, err
	}
	contained, err := p.base.scopes.RangeContained(scope, p.scope)
	return intersects && !contained, err
}

func (p *DirectProofInterpolator) rewriteMover(rewrite ast.ASTHandle) (leftMover, rightMover, common bool, err error) {
	left, right, err := p.rewriteEndpoints(rewrite)
	if err != nil {
		return false, false, false, err
	}
	leftCommon, err := p.termCommon(left)
	if err != nil {
		return false, false, false, err
	}
	rightCommon, err := p.termCommon(right)
	if err != nil {
		return false, false, false, err
	}
	return rightCommon && !leftCommon, leftCommon && !rightCommon, leftCommon && rightCommon, nil
}

func (p *DirectProofInterpolator) rewriteToFormula(rewrite ast.ASTHandle) (ast.ASTHandle, error) {
	_, _, condition, equality, err := p.rewriteFields(rewrite)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.base.manager.MkImplies(condition, equality)
}

func (p *DirectProofInterpolator) flipRewrite(rewrite ast.ASTHandle) (ast.ASTHandle, error) {
	side, position, condition, equality, err := p.rewriteFields(rewrite)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	formula, err := p.rewriteToFormula(rewrite)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	condition, err = p.base.manager.MkImplies(condition, formula)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if side == TermA {
		side = TermB
	} else {
		side = TermA
	}
	return p.makeRewrite(side, position, condition, equality)
}

func (p *DirectProofInterpolator) addRewriteToChain(chain, rewrite ast.ASTHandle) (ast.ASTHandle, error) {
	empty, err := p.base.manager.IsTrue(chain)
	if err != nil || empty {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return p.chainCons(chain, rewrite)
	}
	rest, last, err := p.chainParts(chain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	lastSide, lastPosition, lastCondition, _, err := p.rewriteFields(last)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rewriteSide, rewritePosition, rewriteCondition, _, err := p.rewriteFields(rewrite)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if lastSide == rewriteSide {
		relation, difference, e := p.positionDifference(lastPosition, rewritePosition)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		switch relation {
		case 1:
			_, rewriteRight, e := p.rewriteEndpoints(rewrite)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			absorbed, e := p.updateRewrite(last, difference, rewriteRight, rewriteCondition, true)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			return p.addRewriteToChain(rest, absorbed)
		case 0, -1:
			lastLeft, _, e := p.rewriteEndpoints(last)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			absorbed, e := p.updateRewrite(rewrite, difference, lastLeft, lastCondition, false)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			return p.addRewriteToChain(rest, absorbed)
		default:
			_, lastRightMover, _, e := p.rewriteMover(last)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			rewriteLeftMover, _, _, e := p.rewriteMover(rewrite)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if (rewriteLeftMover && !lastRightMover) || (lastRightMover && !rewriteLeftMover) {
				return p.swapChainRewrites(rest, last, rewrite)
			}
		}
	} else {
		lastLeftMover, lastRightMover, lastCommon, e := p.rewriteMover(last)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		rewriteLeftMover, rewriteRightMover, rewriteCommon, e := p.rewriteMover(rewrite)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if rewriteLeftMover {
			if lastCommon {
				last, e = p.flipRewrite(last)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				chain, e = p.chainCons(rest, last)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				return p.addRewriteToChain(chain, rewrite)
			}
			if !lastLeftMover {
				return p.swapChainRewrites(rest, last, rewrite)
			}
		}
		if lastRightMover {
			if rewriteCommon {
				rewrite, e = p.flipRewrite(rewrite)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				return p.addRewriteToChain(chain, rewrite)
			}
			if !rewriteRightMover {
				return p.swapChainRewrites(rest, last, rewrite)
			}
		}
	}
	return p.chainCons(chain, rewrite)
}

func (p *DirectProofInterpolator) swapChainRewrites(rest, last, rewrite ast.ASTHandle) (ast.ASTHandle, error) {
	chain, err := p.addRewriteToChain(rest, rewrite)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.chainCons(chain, last)
}

func (p *DirectProofInterpolator) concatRewriteChains(first, second ast.ASTHandle) (ast.ASTHandle, error) {
	empty, err := p.base.manager.IsTrue(second)
	if err != nil || empty {
		return first, err
	}
	empty, err = p.base.manager.IsTrue(first)
	if err != nil || empty {
		return second, err
	}
	rest, last, err := p.chainParts(second)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	first, err = p.concatRewriteChains(first, rest)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.addRewriteToChain(first, last)
}

func (p *DirectProofInterpolator) chainPositionAdd(index int64, chain ast.ASTHandle) (ast.ASTHandle, error) {
	empty, err := p.base.manager.IsTrue(chain)
	if err != nil || empty {
		return chain, err
	}
	rest, last, err := p.chainParts(chain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	side, position, condition, equality, err := p.rewriteFields(last)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	position, err = p.positionAdd(index, position)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	last, err = p.makeRewrite(side, position, condition, equality)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rest, err = p.chainPositionAdd(index, rest)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.chainCons(rest, last)
}

func (p *DirectProofInterpolator) chainSideConjunction(side TermSide, chain ast.ASTHandle, formulas bool) (ast.ASTHandle, error) {
	result, err := p.base.manager.Make(OperatorTrue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for {
		empty, e := p.base.manager.IsTrue(chain)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if empty {
			return result, nil
		}
		rest, last, e := p.chainParts(chain)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		rewriteSide, _, condition, equality, e := p.rewriteFields(last)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if rewriteSide == side {
			entry := condition
			if formulas {
				entry = equality
			}
			result, e = p.base.manager.MkAnd(result, entry)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		chain = rest
	}
}

func (p *DirectProofInterpolator) applyRewriteChain(term, chain ast.ASTHandle) (ast.ASTHandle, error) {
	empty, err := p.base.manager.IsTrue(chain)
	if err != nil || empty {
		return term, err
	}
	rest, last, err := p.chainParts(chain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	term, err = p.applyRewriteChain(term, rest)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	_, position, _, _, err := p.rewriteFields(last)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	_, replacement, err := p.rewriteEndpoints(last)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.substituteAtPosition(term, position, replacement)
}

func (p *DirectProofInterpolator) dropRewrites(side TermSide, chain ast.ASTHandle) (kept, remainder ast.ASTHandle, err error) {
	empty, err := p.base.manager.IsTrue(chain)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if !empty {
		rest, last, e := p.chainParts(chain)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		rewriteSide, _, _, _, e := p.rewriteFields(last)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		if rewriteSide == side {
			kept, remainder, e = p.dropRewrites(side, rest)
			if e != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, e
			}
			remainder, e = p.chainCons(remainder, last)
			return kept, remainder, e
		}
	}
	remainder, err = p.base.manager.Make(OperatorTrue)
	return chain, remainder, err
}

func (p *DirectProofInterpolator) chainSideProof(side TermSide, chain ast.ASTHandle) (ast.ASTHandle, error) {
	other := TermA
	if side == TermA {
		other = TermB
	}
	otherConditions, err := p.chainSideConjunction(other, chain, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conditions, err := p.chainSideConjunction(side, chain, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	formulas, err := p.chainSideConjunction(side, chain, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	proof, err := p.base.manager.MkImplies(conditions, formulas)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.base.manager.MkAnd(otherConditions, proof)
}

func (p *DirectProofInterpolator) rewriteAtPosition(rewrite, position ast.ASTHandle) (ast.ASTHandle, error) {
	side, _, condition, equality, err := p.rewriteFields(rewrite)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.makeRewrite(side, position, condition, equality)
}

func (p *DirectProofInterpolator) splitRewriteChain(chain ast.ASTHandle) ([2]ast.ASTHandle, error) {
	truth, err := p.base.manager.Make(OperatorTrue)
	result := [2]ast.ASTHandle{truth, truth}
	if err != nil {
		return result, err
	}
	var split func(ast.ASTHandle) error
	split = func(current ast.ASTHandle) error {
		empty, e := p.base.manager.IsTrue(current)
		if e != nil || empty {
			return e
		}
		rest, last, e := p.chainParts(current)
		if e != nil {
			return e
		}
		if e = split(rest); e != nil {
			return e
		}
		_, position, _, _, e := p.rewriteFields(last)
		if e != nil {
			return e
		}
		if position == p.topPosition {
			left, right, fieldErr := p.rewriteEndpoints(last)
			if fieldErr != nil {
				return fieldErr
			}
			if left == right {
				return nil
			}
			return ProofError{}
		}
		index, positionRest, e := p.positionIndex(position)
		if e != nil || index < 0 || index > 1 {
			return ProofError{}
		}
		last, e = p.rewriteAtPosition(last, positionRest)
		if e != nil {
			return e
		}
		result[index], e = p.chainCons(result[index], last)
		return e
	}
	err = split(chain)
	return result, err
}

func (p *DirectProofInterpolator) headRewriteChain(chain ast.ASTHandle, negation bool) (head, tail ast.ASTHandle, err error) {
	rest, last, err := p.chainParts(chain)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	_, position, _, _, err := p.rewriteFields(last)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	selected := position == p.topPosition
	if !selected && negation {
		_, positionRest, e := p.positionIndex(position)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		selected = positionRest == p.topPosition
	}
	if selected {
		tail, err = p.base.manager.Make(OperatorTrue)
		return chain, tail, err
	}
	empty, err := p.base.manager.IsTrue(rest)
	if err != nil || empty {
		return ast.ASTHandle{}, ast.ASTHandle{}, ProofError{}
	}
	head, tail, err = p.headRewriteChain(rest, negation)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	tail, err = p.chainCons(tail, last)
	return head, tail, err
}

func (p *DirectProofInterpolator) downRewriteChain(chain ast.ASTHandle) (ast.ASTHandle, error) {
	parts, err := p.splitRewriteChain(chain)
	return parts[0], err
}

func (p *DirectProofInterpolator) contradictChains(negativeChain, positiveChain ast.ASTHandle) (ast.ASTHandle, error) {
	equivalence, err := p.isEquivalenceChain(positiveChain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if equivalence {
		head, tail, err := p.headRewriteChain(negativeChain, true)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		tail, err = p.downRewriteChain(tail)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		parts, err := p.splitRewriteChain(tail)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		chain := parts[0]
		chain, err = p.concatRewriteChains(chain, positiveChain)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		second, err := p.reverseChain(parts[1])
		if err != nil {
			return ast.ASTHandle{}, err
		}
		chain, err = p.concatRewriteChains(chain, second)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		chain, err = p.chainPositionAdd(0, chain)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		chain, err = p.chainPositionAdd(0, chain)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		chain, err = p.concatRewriteChains(head, chain)
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
		aConditions, err := p.chainSideConjunction(TermA, chain, false)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		bConditions, err := p.chainSideConjunction(TermB, chain, false)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if headSide == TermB {
			aFormulas, e := p.chainSideConjunction(TermA, chain, true)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			result, e := p.base.manager.MkImplies(aConditions, aFormulas)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			return p.base.manager.MkAnd(result, bConditions)
		}
		bFormulas, err := p.chainSideConjunction(TermB, chain, true)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		bFormulas, err = p.base.manager.MkNot(bFormulas)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		result, err := p.base.manager.MkImplies(aConditions, bFormulas)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return p.base.manager.MkAnd(bConditions, result)
	}
	positiveChain, err = p.reverseChain(positiveChain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	positiveChain, err = p.chainPositionAdd(0, positiveChain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	chain, err := p.concatRewriteChains(negativeChain, positiveChain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	aConditions, err := p.chainSideConjunction(TermA, chain, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	aFormulas, err := p.chainSideConjunction(TermA, chain, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bConditions, err := p.chainSideConjunction(TermB, chain, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := p.base.manager.MkImplies(aConditions, aFormulas)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.base.manager.MkAnd(result, bConditions)
}

func (p *DirectProofInterpolator) simplifyExcludedMiddle(args []ast.ASTHandle) (ast.ASTHandle, error) {
	if len(args) != 3 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	equivalence, err := p.isEquivalence(args[0])
	if err != nil || !equivalence {
		return ast.ASTHandle{}, ProofError{}
	}
	aProof, err := p.base.manager.Make(OperatorTrue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bProof := aProof
	positiveChain, err := p.destructConditionalInequality(args[1], &aProof, &bProof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negativeChain, err := p.destructConditionalInequality(args[2], &aProof, &bProof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := p.contradictChains(negativeChain, positiveChain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err = p.base.manager.MkImplies(bProof, result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.base.manager.MkAnd(aProof, result)
}

func (p *DirectProofInterpolator) simplifyCongruence(args []ast.ASTHandle) (ast.ASTHandle, error) {
	if len(args) != 3 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	position, matched, err := p.base.manager.Numeral(args[1])
	if err != nil || !matched || !position.IsInt() || !position.Num().IsInt64() || position.Sign() < 0 {
		return ast.ASTHandle{}, ProofError{}
	}
	aProof, err := p.base.manager.Make(OperatorTrue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bProof := aProof
	positiveChain, err := p.destructConditionalInequality(args[0], &aProof, &bProof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	positiveChain, err = p.chainPositionAdd(position.Num().Int64(), positiveChain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negativeChain, err := p.destructConditionalInequality(args[2], &aProof, &bProof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := p.contradictChains(negativeChain, positiveChain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err = p.base.manager.MkImplies(bProof, result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.base.manager.MkAnd(aProof, result)
}

func (p *DirectProofInterpolator) simplifyRotatedCongruence(placeholder ast.ASTHandle, proof ast.ASTHandle) (ast.ASTHandle, error) {
	third, err := p.base.manager.Arg(proof, 2)
	if err != nil || third != placeholder {
		return ast.ASTHandle{}, ProofError{}
	}
	chain, err := p.base.manager.Arg(proof, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	empty, err := p.base.manager.IsTrue(chain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if empty {
		return chain, nil
	}
	positionTerm, err := p.base.manager.Arg(proof, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	position, matched, err := p.base.manager.Numeral(positionTerm)
	if err != nil || !matched || !position.IsInt() || !position.Num().IsInt64() || position.Sign() < 0 {
		return ast.ASTHandle{}, ProofError{}
	}
	condition, err := p.base.manager.Make(OperatorTrue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	chain, err = p.separateRewriteCondition(chain, &condition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	chain, err = p.chainPositionAdd(position.Num().Int64(), chain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.base.manager.MkImplies(condition, chain)
}

func (p *DirectProofInterpolator) simplifyRotation(args []ast.ASTHandle) (ast.ASTHandle, error) {
	if len(args) != 2 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	placeholder, err := p.placeholder(args[0])
	if err != nil {
		return ast.ASTHandle{}, err
	}
	op, err := p.base.manager.Operator(args[1])
	if err != nil || op != OperatorUninterpreted {
		return ast.ASTHandle{}, ProofError{}
	}
	declaration, err := p.base.manager.Symbol(args[1])
	if err != nil {
		return ast.ASTHandle{}, err
	}
	switch declaration {
	case p.sum:
		return p.simplifyRotatedSum(args)
	case p.congruence:
		return p.simplifyRotatedCongruence(placeholder, args[1])
	case p.modusPonens:
		return p.simplifyRotatedModusPonens(placeholder, args[1])
	case p.leqToEq:
		return p.simplifyRotatedLeqToEq(placeholder, args[0], args[1])
	case p.eqToLeq:
		return p.simplifyRotatedEqToLeq(placeholder, args[0], args[1])
	default:
		return ast.ASTHandle{}, ProofError{}
	}
}

func (p *DirectProofInterpolator) separateRewriteCondition(term ast.ASTHandle, condition *ast.ASTHandle) (ast.ASTHandle, error) {
	op, err := p.base.manager.Operator(term)
	if err != nil || op != OperatorImplies {
		return term, err
	}
	premise, err := p.base.manager.Arg(term, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	*condition, err = p.base.manager.MkAnd(*condition, premise)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.base.manager.Arg(term, 1)
}

func (p *DirectProofInterpolator) simplifyModusPonens(args []ast.ASTHandle) (ast.ASTHandle, error) {
	if len(args) != 3 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	aProof, err := p.base.manager.Make(OperatorTrue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bProof := aProof
	chain, err := p.modusPonensForward(args, &bProof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	third, err := p.destructConditionalInequality(args[2], &aProof, &bProof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	inequality, err := p.isInequality(third)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	normalInequality, err := p.isNormalInequality(third)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if normalInequality {
		converted, e := p.rewriteChainToNormalInequality(chain, &aProof, &bProof)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		one, e := p.base.manager.MakeIntText("1")
		if e != nil {
			return ast.ASTHandle{}, e
		}
		converted, e = p.sumConditionalInequality(converted, one, third, &aProof, &bProof)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		converted, e = p.base.manager.MkImplies(bProof, converted)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return p.base.manager.MkAnd(aProof, converted)
	}
	if inequality {
		return ast.ASTHandle{}, ProofError{}
	}
	negative, err := p.isNegationChain(chain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var result ast.ASTHandle
	if negative {
		result, err = p.contradictChains(chain, third)
	} else {
		result, err = p.contradictChains(third, chain)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err = p.base.manager.MkImplies(bProof, result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.base.manager.MkAnd(aProof, result)
}

func (p *DirectProofInterpolator) modusPonensForward(args []ast.ASTHandle, condition *ast.ASTHandle) (ast.ASTHandle, error) {
	if len(args) != 3 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	first, err := p.separateRewriteCondition(args[0], condition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	second, err := p.separateRewriteCondition(args[1], condition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	equivalence, err := p.isEquivalenceChain(first)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var chain ast.ASTHandle
	if equivalence {
		parts, e := p.splitRewriteChain(second)
		if e != nil {
			if _, proofError := e.(ProofError); !proofError {
				return ast.ASTHandle{}, e
			}
			chain, e = p.simplifyUnsplitModusPonens(first, second)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		} else {
			chain, e = p.reverseChain(parts[0])
			if e != nil {
				return ast.ASTHandle{}, e
			}
			chain, e = p.concatRewriteChains(chain, first)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			chain, err = p.concatRewriteChains(chain, parts[1])
		}
	} else {
		chain, err = p.concatRewriteChains(first, second)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return chain, nil
}

func (p *DirectProofInterpolator) simplifyUnsplitModusPonens(first, second ast.ASTHandle) (ast.ASTHandle, error) {
	head, tail, err := p.headRewriteChain(second, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	parts, err := p.splitRewriteChain(tail)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	headRest, headRewrite, err := p.chainParts(head)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	side, _, condition, _, err := p.rewriteFields(headRewrite)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, right, err := p.rewriteEndpoints(headRewrite)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	truth, err := p.base.manager.Make(OperatorTrue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negativeLeft, err := p.base.manager.MkNot(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	premise, err := p.base.manager.Make(OperatorIff, truth, negativeLeft)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	premise, err = p.makeRewrite(side, p.topPosition, condition, premise)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	backChain, err := p.chainCons(truth, premise)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	headRest, err = p.reverseChain(headRest)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	headRest, err = p.chainPositionAdd(0, headRest)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	backChain, err = p.concatRewriteChains(backChain, headRest)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	condition, err = p.contradictChains(backChain, first)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if side == TermA {
		condition, err = p.base.manager.MkNot(condition)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	forward, err := p.makeRewrite(side, p.topPosition, condition, right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	first, err = p.chainCons(truth, forward)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	chain, err := p.reverseChain(parts[0])
	if err != nil {
		return ast.ASTHandle{}, err
	}
	chain, err = p.concatRewriteChains(chain, first)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.concatRewriteChains(chain, parts[1])
}

func (p *DirectProofInterpolator) simplifyRotatedModusPonens(placeholder ast.ASTHandle, proof ast.ASTHandle) (ast.ASTHandle, error) {
	args, err := p.base.manager.Args(proof)
	if err != nil || len(args) != 3 {
		return ast.ASTHandle{}, ProofError{}
	}
	if placeholder == args[0] {
		args[1], err = p.simplifySymmetry([]ast.ASTHandle{args[1]})
		if err != nil {
			return ast.ASTHandle{}, err
		}
		equivalence, e := p.isEquivalenceChain(args[2])
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if equivalence {
			args[1], e = p.downRewriteChain(args[1])
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		args[0], args[2] = args[2], args[0]
	}
	if placeholder != args[2] {
		return ast.ASTHandle{}, ProofError{}
	}
	condition, err := p.base.manager.Make(OperatorTrue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	chain, err := p.modusPonensForward(args, &condition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.base.manager.MkImplies(condition, chain)
}

func (p *DirectProofInterpolator) isEquivalenceChain(chain ast.ASTHandle) (bool, error) {
	for {
		empty, err := p.base.manager.IsTrue(chain)
		if err != nil || empty {
			return empty, err
		}
		rest, last, err := p.chainParts(chain)
		if err != nil {
			return false, err
		}
		restEmpty, err := p.base.manager.IsTrue(rest)
		if err != nil {
			return false, err
		}
		if restEmpty {
			equality, e := p.base.manager.Arg(last, 2)
			if e != nil {
				return false, e
			}
			left, e := p.base.manager.Arg(equality, 0)
			if e != nil {
				return false, e
			}
			leftTrue, e := p.base.manager.IsTrue(left)
			return !leftTrue, e
		}
		chain = rest
	}
}

func (p *DirectProofInterpolator) isNegationChain(chain ast.ASTHandle) (bool, error) {
	for {
		empty, err := p.base.manager.IsTrue(chain)
		if err != nil || empty {
			return false, err
		}
		rest, last, err := p.chainParts(chain)
		if err != nil {
			return false, err
		}
		restEmpty, err := p.base.manager.IsTrue(rest)
		if err != nil {
			return false, err
		}
		if restEmpty {
			_, _, _, equality, e := p.rewriteFields(last)
			if e != nil {
				return false, e
			}
			right, e := p.base.manager.Arg(equality, 1)
			if e != nil {
				return false, e
			}
			op, e := p.base.manager.Operator(right)
			return op == OperatorNot, e
		}
		chain = rest
	}
}

func (p *DirectProofInterpolator) reverseBinary(term ast.ASTHandle) (ast.ASTHandle, error) {
	op, err := p.base.manager.Operator(term)
	if err != nil || (op != OperatorEqual && op != OperatorIff) {
		return ast.ASTHandle{}, ProofError{}
	}
	left, err := p.base.manager.Arg(term, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := p.base.manager.Arg(term, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.base.manager.Make(op, right, left)
}

func (p *DirectProofInterpolator) commuteNegationChain(chain ast.ASTHandle) (ast.ASTHandle, error) {
	empty, err := p.base.manager.IsTrue(chain)
	if err != nil || empty {
		return chain, err
	}
	rest, last, err := p.chainParts(chain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	side, position, condition, equality, err := p.rewriteFields(last)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	restEmpty, err := p.base.manager.IsTrue(rest)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if restEmpty {
		left, e := p.base.manager.Arg(equality, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		right, e := p.base.manager.Arg(equality, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		rightOp, e := p.base.manager.Operator(right)
		if e != nil || rightOp != OperatorNot {
			return ast.ASTHandle{}, ProofError{}
		}
		body, e := p.base.manager.Arg(right, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		body, e = p.reverseBinary(body)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		right, e = p.base.manager.Make(OperatorNot, body)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		equalityOp, e := p.base.manager.Operator(equality)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		equality, e = p.base.manager.Make(equalityOp, left, right)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		last, e = p.makeRewrite(side, position, condition, equality)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return p.chainCons(rest, last)
	}
	if position == p.topPosition {
		return ast.ASTHandle{}, ProofError{}
	}
	index, positionRest, err := p.positionIndex(position)
	if err != nil || index != 0 {
		return ast.ASTHandle{}, ProofError{}
	}
	if positionRest == p.topPosition {
		left, e := p.base.manager.Arg(equality, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		right, e := p.base.manager.Arg(equality, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		left, e = p.reverseBinary(left)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		right, e = p.reverseBinary(right)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		equality, e = p.base.manager.Make(OperatorIff, left, right)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		last, e = p.makeRewrite(side, position, condition, equality)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	} else {
		innerIndex, innerRest, e := p.positionIndex(positionRest)
		if e != nil || (innerIndex != 0 && innerIndex != 1) {
			return ast.ASTHandle{}, ProofError{}
		}
		innerIndex = 1 - innerIndex
		positionRest, e = p.positionAdd(innerIndex, innerRest)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		position, e = p.positionAdd(0, positionRest)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		last, e = p.makeRewrite(side, position, condition, equality)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	rest, err = p.commuteNegationChain(rest)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.chainCons(rest, last)
}

func (p *DirectProofInterpolator) simplifySymmetry(args []ast.ASTHandle) (ast.ASTHandle, error) {
	if len(args) != 1 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	truth, err := p.base.manager.Make(OperatorTrue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	condition := truth
	chain := args[0]
	op, err := p.base.manager.Operator(chain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if op == OperatorTrue {
		return truth, nil
	}
	if op == OperatorImplies {
		condition, err = p.base.manager.Arg(chain, 0)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		chain, err = p.base.manager.Arg(chain, 1)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	equivalence, err := p.isEquivalenceChain(chain)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if equivalence {
		chain, err = p.reverseChain(chain)
	} else {
		negation, e := p.isNegationChain(chain)
		if e != nil || !negation {
			return ast.ASTHandle{}, ProofError{}
		}
		chain, err = p.commuteNegationChain(chain)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.base.manager.MkImplies(condition, chain)
}

func (p *DirectProofInterpolator) makeLocalRewrite(side TermSide, formula ast.ASTHandle) (ast.ASTHandle, error) {
	equivalence, err := p.isEquivalence(formula)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !equivalence {
		truth, e := p.base.manager.Make(OperatorTrue)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		formula, err = p.base.manager.Make(OperatorIff, truth, formula)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	truth, err := p.base.manager.Make(OperatorTrue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rewrite, err := p.makeRewrite(side, p.topPosition, truth, formula)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.chainCons(truth, rewrite)
}

func (p *DirectProofInterpolator) trivialInterpolation(rule ast.ASTHandle, premises []ast.ASTHandle, inputMask int) (ast.ASTHandle, error) {
	proofs := make([]ast.ASTHandle, len(premises))
	conjuncts := make([]ast.ASTHandle, 0, len(premises))
	mask := 0
	for i, premise := range premises {
		side, err := p.TermSide(premise)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if side == TermA || side == TermB {
			proofs[i], err = p.makeLocalRewrite(side, premise)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			continue
		}
		proofs[i], err = p.placeholder(premise)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if inputMask&(1<<i) != 0 {
			mask |= 1 << len(conjuncts)
		}
		conjuncts = append(conjuncts, premise)
	}
	proof, err := p.makeSymbol(rule, proofs...)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.makeContraNode(proof, conjuncts, mask)
}
