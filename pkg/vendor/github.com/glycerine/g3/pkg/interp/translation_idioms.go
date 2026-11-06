package interp

// Source checkpoint: src/interp/iz3translate.cpp retained proof idiom
// recognition before the ordinary rule dispatcher.

import (
	"math"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *FullTranslation) rewriteClause(clause, rewrite ast.ASTHandle, direct *DirectProofInterpolator, memo *translationMemo) (ast.ASTHandle, error) {
	rule, err := t.base.manager.ProofRule(rewrite)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	switch rule {
	case ast.ASTPrMonotonicity:
		count, e := t.base.manager.NumPremises(rewrite)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		for i := uint32(0); i < count; i++ {
			premise, e := t.base.manager.Premise(rewrite, i)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			proof, e := t.translateMain(premise, false, direct, memo)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			equivalence, e := t.base.manager.Conclusion(premise)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			clause, e = direct.MakeModusPonens(equivalence, clause, proof)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		return clause, nil
	case ast.ASTPrTransitivity:
		first, e := t.base.manager.Premise(rewrite, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		clause, e = t.rewriteClause(clause, first, direct, memo)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		second, e := t.base.manager.Premise(rewrite, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return t.rewriteClause(clause, second, direct, memo)
	case ast.ASTPrRewrite:
		return clause, nil
	default:
		return ast.ASTHandle{}, UnsupportedProofError{Proof: rewrite}
	}
}

func (t *FullTranslation) isEqualityPropagation(proof ast.ASTHandle) (bool, error) {
	rule, err := t.base.manager.ProofRule(proof)
	if err != nil || rule != ast.ASTPrTheoryLemma {
		return false, err
	}
	theory, err := t.base.manager.TheoryLemmaTheory(proof)
	if err != nil || theory != ArithTheory {
		return false, err
	}
	kind, err := t.base.manager.TheoryLemmaKind(proof)
	return kind == EqPropagateKind, err
}

func (t *FullTranslation) collectEqualityPropagation(proof ast.ASTHandle, sourcePremises, translatedPremises *[]ast.ASTHandle, equalityPremise *ast.ASTHandle, direct *DirectProofInterpolator, memo *translationMemo) error {
	rule, err := t.base.manager.ProofRule(proof)
	if err != nil {
		return err
	}
	if rule == ast.ASTPrTransitivity {
		second, e := t.base.manager.Premise(proof, 1)
		if e != nil {
			return e
		}
		equalityPropagation, e := t.isEqualityPropagation(second)
		if e != nil {
			return e
		}
		if equalityPropagation {
			first, e := t.base.manager.Premise(proof, 0)
			if e != nil {
				return e
			}
			if e = t.collectEqualityPropagation(first, sourcePremises, translatedPremises, equalityPremise, direct, memo); e != nil {
				return e
			}
			var ignored ast.ASTHandle
			return t.collectEqualityPropagation(second, sourcePremises, translatedPremises, &ignored, direct, memo)
		}
	}
	if equalityPropagation, e := t.isEqualityPropagation(proof); e != nil {
		return e
	} else if equalityPropagation {
		count, e := t.base.manager.NumPremises(proof)
		if e != nil {
			return e
		}
		for i := uint32(0); i < count; i++ {
			premise, e := t.base.manager.Premise(proof, i)
			if e != nil {
				return e
			}
			translated, e := t.translateMain(premise, false, direct, memo)
			if e != nil {
				return e
			}
			*sourcePremises = append(*sourcePremises, premise)
			*translatedPremises = append(*translatedPremises, translated)
		}
		return nil
	}
	*equalityPremise = proof
	return nil
}

func (t *FullTranslation) combineEqualityPropagation(proof ast.ASTHandle, direct *DirectProofInterpolator, memo *translationMemo) (ast.ASTHandle, error) {
	var sourcePremises, translatedPremises []ast.ASTHandle
	var equalityPremise ast.ASTHandle
	if err := t.collectEqualityPropagation(proof, &sourcePremises, &translatedPremises, &equalityPremise, direct, memo); err != nil {
		return ast.ASTHandle{}, err
	}
	conclusion, err := t.base.manager.Conclusion(proof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	propagatedConclusion := conclusion
	if !equalityPremise.IsNull() {
		firstConclusion, e := t.base.manager.Conclusion(equalityPremise)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		middle, e := t.base.manager.Arg(firstConclusion, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		right, e := t.base.manager.Arg(conclusion, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		propagatedConclusion, e = t.base.manager.Make(OperatorEqual, middle, right)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	result, err := t.translateEqualityPropagation(propagatedConclusion, sourcePremises, translatedPremises, direct)
	if err != nil || equalityPremise.IsNull() {
		return result, err
	}
	firstProof, err := t.translateMain(equalityPremise, false, direct, memo)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	firstConclusion, err := t.base.manager.Conclusion(equalityPremise)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err := t.base.manager.Arg(firstConclusion, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	middle, err := t.base.manager.Arg(firstConclusion, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := t.base.manager.Arg(conclusion, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return direct.MakeTransitivity(left, middle, right, firstProof, result)
}

func (t *FullTranslation) proofLiterals(conclusion ast.ASTHandle, expectClause bool) ([]ast.ASTHandle, error) {
	if expectClause {
		return t.clauseLiterals(conclusion)
	}
	return []ast.ASTHandle{conclusion}, nil
}

func (t *FullTranslation) translateProofIdiom(proof, conclusion ast.ASTHandle, rule int32, expectClause bool, direct *DirectProofInterpolator, memo *translationMemo) (ast.ASTHandle, bool, error) {
	if rule == ast.ASTPrModusPonens {
		first, err := t.base.manager.Premise(proof, 0)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		second, err := t.base.manager.Premise(proof, 1)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		firstRule, err := t.base.manager.ProofRule(first)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		secondRule, err := t.base.manager.ProofRule(second)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if firstRule == ast.ASTPrQuantInst {
			locality, e := t.Locality(second)
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			if locality == math.MaxInt {
				literals, e := t.proofLiterals(conclusion, expectClause)
				if e != nil {
					return ast.ASTHandle{}, false, e
				}
				result, e := direct.MakeAxiom(literals)
				return result, true, e
			}
		}
		if expectClause {
			conclusionOp, e := t.base.manager.Operator(conclusion)
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			firstConclusion, e := t.base.manager.Conclusion(first)
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			firstOp, e := t.base.manager.Operator(firstConclusion)
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			if conclusionOp == OperatorOr && firstOp == OperatorOr {
				clause, e := t.translateMain(first, true, direct, memo)
				if e != nil {
					return ast.ASTHandle{}, false, e
				}
				result, e := t.rewriteClause(clause, second, direct, memo)
				return result, true, e
			}
		}
		if secondRule == ast.ASTPrMonotonicity {
			count, e := t.base.manager.NumPremises(second)
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			if count == 1 {
				commutativity, e := t.base.manager.Premise(second, 0)
				if e != nil {
					return ast.ASTHandle{}, false, e
				}
				commutativityRule, e := t.base.manager.ProofRule(commutativity)
				if e != nil {
					return ast.ASTHandle{}, false, e
				}
				if commutativityRule == ast.ASTPrCommutativity {
					antecedent, e := t.translateMain(first, false, direct, memo)
					if e != nil {
						return ast.ASTHandle{}, false, e
					}
					commutedConclusion, e := t.base.manager.Conclusion(commutativity)
					if e != nil {
						return ast.ASTHandle{}, false, e
					}
					eq0, e := t.base.manager.Arg(commutedConclusion, 0)
					if e != nil {
						return ast.ASTHandle{}, false, e
					}
					eq1, e := t.base.manager.Arg(commutedConclusion, 1)
					if e != nil {
						return ast.ASTHandle{}, false, e
					}
					hypothesis, e := direct.MakeHypothesis(eq1)
					if e != nil {
						return ast.ASTHandle{}, false, e
					}
					symmetry, e := direct.MakeSymmetry(eq0, eq1, hypothesis)
					if e != nil {
						return ast.ASTHandle{}, false, e
					}
					result, e := direct.MakeResolution(eq0, nil, antecedent, symmetry)
					return result, true, e
				}
			}
		}
		if secondRule == ast.ASTPrRewrite && firstRule == ast.ASTPrTransitivity {
			iffFalse, e := t.base.manager.Premise(first, 1)
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			iffFalseRule, e := t.base.manager.ProofRule(iffFalse)
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			if iffFalseRule == ast.ASTPrIffFalse {
				conclusionOp, e := t.base.manager.Operator(conclusion)
				if e != nil {
					return ast.ASTHandle{}, false, e
				}
				firstConclusion, e := t.base.manager.Conclusion(first)
				if e != nil {
					return ast.ASTHandle{}, false, e
				}
				negated, e := t.base.manager.Arg(conclusion, 0)
				if conclusionOp == OperatorNot && e == nil {
					firstLeft, e := t.base.manager.Arg(firstConclusion, 0)
					if e != nil {
						return ast.ASTHandle{}, false, e
					}
					if negated == firstLeft {
						antecedent1Source, e := t.base.manager.Premise(first, 0)
						if e != nil {
							return ast.ASTHandle{}, false, e
						}
						antecedent2Source, e := t.base.manager.Premise(iffFalse, 0)
						if e != nil {
							return ast.ASTHandle{}, false, e
						}
						antecedent1, e := t.translateMain(antecedent1Source, false, direct, memo)
						if e != nil {
							return ast.ASTHandle{}, false, e
						}
						antecedent2, e := t.translateMain(antecedent2Source, false, direct, memo)
						if e != nil {
							return ast.ASTHandle{}, false, e
						}
						antecedent1Conclusion, e := t.base.manager.Conclusion(antecedent1Source)
						if e != nil {
							return ast.ASTHandle{}, false, e
						}
						eq0, e := t.base.manager.Arg(antecedent1Conclusion, 0)
						if e != nil {
							return ast.ASTHandle{}, false, e
						}
						eq1, e := t.base.manager.Arg(antecedent1Conclusion, 1)
						if e != nil {
							return ast.ASTHandle{}, false, e
						}
						symmetricConclusion, e := t.base.manager.Make(OperatorIff, eq1, eq0)
						if e != nil {
							return ast.ASTHandle{}, false, e
						}
						symmetric, e := direct.MakeSymmetry(symmetricConclusion, antecedent1Conclusion, antecedent1)
						if e != nil {
							return ast.ASTHandle{}, false, e
						}
						notEq1, e := t.base.manager.MkNot(eq1)
						if e != nil {
							return ast.ASTHandle{}, false, e
						}
						notEq0, e := t.base.manager.MkNot(eq0)
						if e != nil {
							return ast.ASTHandle{}, false, e
						}
						congruenceConclusion, e := t.base.manager.Make(OperatorIff, notEq1, notEq0)
						if e != nil {
							return ast.ASTHandle{}, false, e
						}
						congruence, e := direct.MakeCongruence(symmetricConclusion, congruenceConclusion, symmetric)
						if e != nil {
							return ast.ASTHandle{}, false, e
						}
						result, e := direct.MakeModusPonens(congruenceConclusion, antecedent2, congruence)
						return result, true, e
					}
				}
			}
		}
	}
	if rule == ast.ASTPrModusPonensOEq {
		first, err := t.base.manager.Premise(proof, 0)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		firstConclusion, err := t.base.manager.Conclusion(first)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if firstConclusion == conclusion {
			result, e := t.translateMain(first, expectClause, direct, memo)
			return result, true, e
		}
		if expectClause {
			op, e := t.base.manager.Operator(conclusion)
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			if op == OperatorOr {
				second, e := t.base.manager.Premise(proof, 1)
				if e != nil {
					return ast.ASTHandle{}, false, e
				}
				clause, e := t.translateMain(first, true, direct, memo)
				if e != nil {
					return ast.ASTHandle{}, false, e
				}
				result, e := t.rewriteClause(clause, second, direct, memo)
				return result, true, e
			}
		}
	}
	if rule == ast.ASTPrTransitivity {
		second, err := t.base.manager.Premise(proof, 1)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		equalityPropagation, err := t.isEqualityPropagation(second)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if equalityPropagation {
			result, e := t.combineEqualityPropagation(proof, direct, memo)
			return result, true, e
		}
	}
	return ast.ASTHandle{}, false, nil
}
