package interp

// Source checkpoint: src/interp/iz3translate.cpp retained proof-rule
// dispatcher and final precomputed-lemma construction.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type UnsupportedProofError struct{ Proof ast.ASTHandle }

func (UnsupportedProofError) Error() string { return "unsupported interpolation proof" }

type translationMemo struct {
	clause map[ast.ASTHandle]ast.ASTHandle
	term   map[ast.ASTHandle]ast.ASTHandle
}

func newTranslationMemo() *translationMemo {
	return &translationMemo{clause: make(map[ast.ASTHandle]ast.ASTHandle), term: make(map[ast.ASTHandle]ast.ASTHandle)}
}

func (t *FullTranslation) translateUnitResolution(proof ast.ASTHandle, direct *DirectProofInterpolator, memo *translationMemo) (ast.ASTHandle, error) {
	first, err := t.base.manager.Premise(proof, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	interpolation, err := t.translateMain(first, true, direct, memo)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	firstConclusion, err := t.base.manager.Conclusion(first)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	count, err := t.base.manager.NumPremises(proof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var clause []ast.ASTHandle
	if count == 2 {
		second, e := t.base.manager.Premise(proof, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		secondConclusion, e := t.base.manager.Conclusion(second)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		negated, e := t.base.manager.MkNot(secondConclusion)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if firstConclusion == negated {
			clause = []ast.ASTHandle{firstConclusion}
		}
	}
	if clause == nil {
		clause, err = t.clauseLiterals(firstConclusion)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	for position := uint32(1); position < count; position++ {
		antecedent, e := t.base.manager.Premise(proof, position)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		literal, e := t.base.manager.Conclusion(antecedent)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		absolute := literal
		negative := false
		if op, operatorErr := t.base.manager.Operator(literal); operatorErr != nil {
			return ast.ASTHandle{}, operatorErr
		} else if op == OperatorNot {
			negative = true
			absolute, e = t.base.manager.Arg(literal, 0)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		other, e := t.translateMain(antecedent, false, direct, memo)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		negativeProof, positiveProof := interpolation, other
		if negative {
			negativeProof, positiveProof = positiveProof, negativeProof
		}
		negatedLiteral, e := t.base.manager.MkNot(literal)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		clause, e = t.resolveClause(negatedLiteral, clause, []ast.ASTHandle{literal})
		if e != nil {
			return ast.ASTHandle{}, e
		}
		interpolation, e = direct.MakeResolution(absolute, clause, negativeProof, positiveProof)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	return interpolation, nil
}

func (t *FullTranslation) translateMain(proof ast.ASTHandle, expectClause bool, direct *DirectProofInterpolator, memo *translationMemo) (ast.ASTHandle, error) {
	cache := memo.term
	if expectClause {
		cache = memo.clause
	}
	if result, found := cache[proof]; found {
		return result, nil
	}
	frame, err := t.Locality(proof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conclusion, err := t.base.manager.Conclusion(proof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if frame != -1 {
		if frame >= t.frames {
			frame = t.frames - 1
		}
		formulas := []ast.ASTHandle{conclusion}
		if expectClause {
			formulas, err = t.clauseLiterals(conclusion)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		hypotheses, err := t.proofHypotheses(proof)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		for hypothesis := range hypotheses {
			negated, e := t.base.manager.MkNot(hypothesis)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			formulas = append(formulas, negated)
		}
		result, err := direct.MakeAssumption(frame, formulas)
		if err == nil {
			cache[proof] = result
		}
		return result, err
	}
	rule, err := t.base.manager.ProofRule(proof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if rule == ast.ASTPrUnitResolution {
		result, e := t.translateUnitResolution(proof, direct, memo)
		if e == nil {
			cache[proof] = result
		}
		return result, e
	}
	if rule == ast.ASTPrLemma {
		premise, e := t.base.manager.Premise(proof, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		result, e := t.translateMain(premise, true, direct, memo)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if !expectClause {
			formulas := []ast.ASTHandle{conclusion}
			hypotheses, e := t.proofHypotheses(proof)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			for hypothesis := range hypotheses {
				negated, e := t.base.manager.MkNot(hypothesis)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				formulas = append(formulas, negated)
			}
			result, e = direct.MakeContra(result, formulas)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		cache[proof] = result
		return result, nil
	}
	if result, handled, e := t.translateProofIdiom(proof, conclusion, rule, expectClause, direct, memo); e != nil {
		return ast.ASTHandle{}, e
	} else if handled {
		cache[proof] = result
		return result, nil
	}
	count, err := t.base.manager.NumPremises(proof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	premises := make([]ast.ASTHandle, count)
	premiseConclusions := make([]ast.ASTHandle, count)
	for i := uint32(0); i < count; i++ {
		premise, e := t.base.manager.Premise(proof, i)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		premiseConclusions[i], e = t.base.manager.Conclusion(premise)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		premises[i], e = t.translateMain(premise, false, direct, memo)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	commuted := make([]bool, len(premises))
	for i := range premises {
		commuted[i], err = t.isCommuted(premises[i])
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if commuted[i] && rule != ast.ASTPrTransitivity && rule != ast.ASTPrModusPonens && rule != ast.ASTPrSymmetry && rule != ast.ASTPrMonotonicity {
			return ast.ASTHandle{}, UnsupportedProofError{Proof: proof}
		}
	}
	var result ast.ASTHandle
	switch rule {
	case ast.ASTPrTransitivity:
		if len(premises) < 2 {
			return ast.ASTHandle{}, ProofError{}
		}
		if commuted[0] || commuted[1] {
			result, err = t.makeCommutedTransitivity(proof, premises, direct, memo)
			break
		}
		x, e := t.base.manager.Arg(premiseConclusions[0], 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		y, e := t.base.manager.Arg(premiseConclusions[0], 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		z, e := t.base.manager.Arg(premiseConclusions[1], 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		result, e = direct.MakeTransitivity(x, y, z, premises[0], premises[1])
		if e != nil {
			return ast.ASTHandle{}, e
		}
		for i := 2; i < len(premises); i++ {
			y = z
			z, e = t.base.manager.Arg(premiseConclusions[i], 1)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			result, e = direct.MakeTransitivity(x, y, z, result, premises[i])
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
	case ast.ASTPrTransitivityStar:
		if len(premises) < 2 {
			return ast.ASTHandle{}, ProofError{}
		}
		x, e := t.base.manager.Arg(premiseConclusions[0], 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		y, e := t.base.manager.Arg(premiseConclusions[0], 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		z, e := t.base.manager.Arg(premiseConclusions[1], 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		result, err = direct.MakeTransitivity(x, y, z, premises[0], premises[1])
		for i := 2; err == nil && i < len(premises); i++ {
			y = z
			z, err = t.base.manager.Arg(premiseConclusions[i], 1)
			if err == nil {
				result, err = direct.MakeTransitivity(x, y, z, result, premises[i])
			}
		}
	case ast.ASTPrQuantIntro, ast.ASTPrMonotonicity:
		if rule == ast.ASTPrMonotonicity && len(commuted) == 1 && commuted[0] {
			left, e := t.base.manager.Arg(conclusion, 0)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			op, e := t.base.manager.Operator(left)
			if e != nil || op != OperatorNot {
				if e != nil {
					return ast.ASTHandle{}, e
				}
				return ast.ASTHandle{}, UnsupportedProofError{Proof: proof}
			}
			result, err = t.makeCommutedMonotonicity(premises[0], direct)
		} else {
			result, err = direct.MakeCongruences(premiseConclusions, conclusion, premises)
		}
	case ast.ASTPrReflexivity:
		result, err = direct.MakeReflexivity(conclusion)
	case ast.ASTPrSymmetry:
		if len(premises) != 1 {
			return ast.ASTHandle{}, ProofError{}
		}
		if commuted[0] {
			result, err = t.makeCommutedSymmetry(premises[0], direct)
		} else {
			result, err = direct.MakeSymmetry(conclusion, premiseConclusions[0], premises[0])
		}
	case ast.ASTPrModusPonens:
		if len(premises) != 2 {
			return ast.ASTHandle{}, ProofError{}
		}
		if commuted[1] {
			result, err = t.makeCommutedModusPonens(proof, premises, direct)
		} else {
			result, err = direct.MakeModusPonens(premiseConclusions[1], premises[0], premises[1])
		}
	case ast.ASTPrTheoryLemma:
		result, err = t.translateTheoryLemma(proof, conclusion, premises, premiseConclusions, direct)
	case ast.ASTPrHypothesis:
		result, err = direct.MakeHypothesis(conclusion)
	case ast.ASTPrQuantInst, ast.ASTPrDefAxiom:
		var literals []ast.ASTHandle
		if expectClause {
			literals, err = t.clauseLiterals(conclusion)
		} else {
			literals = []ast.ASTHandle{conclusion}
		}
		if err == nil {
			result, err = direct.MakeAxiom(literals)
		}
	case ast.ASTPrIffTrue:
		if len(premises) != 1 {
			return ast.ASTHandle{}, ProofError{}
		}
		result = premises[0]
	case ast.ASTPrCommutativity:
		left, e := t.base.manager.Arg(conclusion, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		equivalence, e := t.base.manager.Make(OperatorIff, left, left)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		reflexivity, e := direct.MakeReflexivity(equivalence)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		result, err = t.makeCommuted(reflexivity, equivalence)
	case ast.ASTPrIffFalse:
		local, e := t.base.IsLocal(conclusion)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if !local || len(premises) != 1 {
			return ast.ASTHandle{}, UnsupportedProofError{Proof: proof}
		}
		result = premises[0]
	case ast.ASTPrNotOrElim, ast.ASTPrAndElim:
		if len(premises) != 1 {
			return ast.ASTHandle{}, ProofError{}
		}
		negatedPivot, e := t.base.manager.MkNot(premiseConclusions[0])
		if e != nil {
			return ast.ASTHandle{}, e
		}
		axiom, e := direct.MakeAxiom([]ast.ASTHandle{negatedPivot, conclusion})
		if e != nil {
			return ast.ASTHandle{}, e
		}
		result, err = direct.MakeResolution(premiseConclusions[0], []ast.ASTHandle{conclusion}, axiom, premises[0])
	default:
		return ast.ASTHandle{}, UnsupportedProofError{Proof: proof}
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	cache[proof] = result
	return result, nil
}

func (t *FullTranslation) findStore(term ast.ASTHandle) (ast.ASTHandle, bool, error) {
	op, err := t.base.manager.Operator(term)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if op == OperatorStore {
		return term, true, nil
	}
	arguments, err := t.base.manager.Args(term)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	for _, argument := range arguments {
		if store, found, e := t.findStore(argument); e != nil || found {
			return store, found, e
		}
	}
	return ast.ASTHandle{}, false, nil
}

func (t *FullTranslation) translateTheoryLemma(proof, conclusion ast.ASTHandle, premises, premiseConclusions []ast.ASTHandle, direct *DirectProofInterpolator) (ast.ASTHandle, error) {
	theory, err := t.base.manager.TheoryLemmaTheory(proof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if theory == ArrayTheory {
		literals, e := t.clauseLiterals(conclusion)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		store, found, e := t.findStore(conclusion)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if !found {
			return direct.MakeAxiom(literals)
		}
		scope, e := t.base.ASTScope(store)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return direct.MakeAxiomInRange(literals, scope)
	}
	if theory != ArithTheory {
		return ast.ASTHandle{}, UnsupportedProofError{Proof: proof}
	}
	kind, err := t.base.manager.TheoryLemmaKind(proof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	switch kind {
	case FarkasKind:
		coefficients, e := t.base.manager.FarkasCoefficientTerms(proof)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		proofs := append([]ast.ASTHandle(nil), premises...)
		conclusions := append([]ast.ASTHandle(nil), premiseConclusions...)
		if len(proofs) == 0 {
			arguments, e := t.base.manager.Args(conclusion)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if len(coefficients) != len(arguments) {
				return ast.ASTHandle{}, UnsupportedProofError{Proof: proof}
			}
			for _, argument := range arguments {
				literal, e := t.base.manager.MkNot(argument)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				conclusions = append(conclusions, literal)
				hypothesis, e := direct.MakeHypothesis(literal)
				if e != nil {
					return ast.ASTHandle{}, e
				}
				proofs = append(proofs, hypothesis)
			}
		}
		return direct.MakeFarkas(conclusion, proofs, conclusions, coefficients)
	case Leq2EqKind:
		equality, e := t.base.manager.Arg(conclusion, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		x, e := t.base.manager.Arg(equality, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		y, e := t.base.manager.Arg(equality, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		firstNegative, e := t.base.manager.Arg(conclusion, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		first, e := t.base.manager.Arg(firstNegative, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		secondNegative, e := t.base.manager.Arg(conclusion, 2)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		second, e := t.base.manager.Arg(secondNegative, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return direct.MakeLeqToEq(x, y, first, second)
	case Eq2LeqKind:
		negativeEquality, e := t.base.manager.Arg(conclusion, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		equality, e := t.base.manager.Arg(negativeEquality, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		x, e := t.base.manager.Arg(equality, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		y, e := t.base.manager.Arg(equality, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		inequality, e := t.base.manager.Arg(conclusion, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return direct.MakeEqToLeq(x, y, inequality)
	case GCDTestKind:
		return t.translateGCDTest(proof, premises, direct)
	case AssignBoundsKind:
		return t.translateAssignBounds(proof, conclusion, premises, premiseConclusions, direct)
	case GomoryCutKind:
		return t.translateGomoryCut(proof, conclusion, premises, premiseConclusions, direct)
	case EqPropagateKind:
		sourcePremises := make([]ast.ASTHandle, len(premises))
		for i := range sourcePremises {
			sourcePremises[i], err = t.base.manager.Premise(proof, uint32(i))
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		return t.translateEqualityPropagation(conclusion, sourcePremises, premises, direct)
	case ArithMysteryKind:
		sourcePremises := make([]ast.ASTHandle, len(premises))
		for i := range sourcePremises {
			sourcePremises[i], err = t.base.manager.Premise(proof, uint32(i))
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		return t.reconstructFarkas(sourcePremises, premises, conclusion, direct)
	default:
		return ast.ASTHandle{}, UnsupportedProofError{Proof: proof}
	}
}

func (t *FullTranslation) gcdToDivisionRule(proof ast.ASTHandle, positive bool, coefficients []*big.Rat, premises []ast.ASTHandle, direct *DirectProofInterpolator) (ast.ASTHandle, ast.ASTHandle, error) {
	var selectedProofs, selectedCoefficients, selectedConclusions []ast.ASTHandle
	start := 0
	if !positive {
		start = 1
	}
	for i := start; i < len(coefficients); i += 2 {
		coefficient := coefficients[i]
		selected := i
		value := new(big.Rat).Set(coefficient)
		if coefficient.Sign() < 0 {
			if i%2 == 0 {
				selected = i + 1
			} else {
				selected = i - 1
			}
			if selected < 0 || selected >= len(coefficients) {
				return ast.ASTHandle{}, ast.ASTHandle{}, ProofError{}
			}
			value.Neg(coefficients[selected])
		}
		if coefficient.Sign() == 0 {
			continue
		}
		selectedPremise, err := t.base.manager.Premise(proof, uint32(selected))
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		conclusion, err := t.base.manager.Conclusion(selectedPremise)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		term, err := t.base.manager.MakeInt(value)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		selectedProofs = append(selectedProofs, premises[selected])
		selectedCoefficients = append(selectedCoefficients, term)
		selectedConclusions = append(selectedConclusions, conclusion)
	}
	combined, err := t.base.manager.SumInequalities(selectedCoefficients, selectedConclusions, false)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if len(coefficients) != 0 && coefficients[0].Sign() == 0 {
		if len(premises) < 2 {
			return ast.ASTHandle{}, ast.ASTHandle{}, ProofError{}
		}
		firstPremise, e := t.base.manager.Premise(proof, 0)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		firstConclusion, e := t.base.manager.Conclusion(firstPremise)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		variable, lower, e := t.BoundedVariable(firstConclusion)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		value, e := t.TermCoefficient(combined, variable)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		extra := 0
		if (value.Sign() > 0) != lower {
			extra = 1
		}
		value.Abs(value)
		term, e := t.base.manager.MakeInt(value)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		extraPremise, e := t.base.manager.Premise(proof, uint32(extra))
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		extraConclusion, e := t.base.manager.Conclusion(extraPremise)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		selectedProofs = append(selectedProofs, premises[extra])
		selectedCoefficients = append(selectedCoefficients, term)
		selectedConclusions = append(selectedConclusions, extraConclusion)
		combined, e = t.base.manager.SumInequalities(selectedCoefficients, selectedConclusions, false)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, e
		}
	}
	combined, err = t.NormalizeInequality(combined)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	negative, err := t.base.manager.MkNot(combined)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	hypothesis, err := direct.MakeHypothesis(negative)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	selectedProofs = append(selectedProofs, hypothesis)
	selectedConclusions = append(selectedConclusions, negative)
	one, err := t.base.manager.MakeIntText("1")
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	selectedCoefficients = append(selectedCoefficients, one)
	falsehood, err := t.base.manager.Make(OperatorFalse)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	interpolation, err := direct.MakeFarkas(falsehood, selectedProofs, selectedConclusions, selectedCoefficients)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	left, err := t.base.manager.Arg(combined, 0)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	right, err := t.base.manager.Arg(combined, 1)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	divisor, err := t.GCDOfCoefficients(left)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	left, err = t.ownedIntegerDivision(left, divisor)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	right, err = t.ownedIntegerDivision(right, divisor)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	left, err = t.ownedSimplify(left)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	right, err = t.ownedSimplify(right)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	op, err := t.base.manager.Operator(combined)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	cutConclusion, err := t.base.manager.Make(op, left, right)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	interpolation, err = direct.MakeCutRule(combined, divisor, cutConclusion, interpolation)
	return interpolation, cutConclusion, err
}

func (t *FullTranslation) translateGCDTest(proof ast.ASTHandle, premises []ast.ASTHandle, direct *DirectProofInterpolator) (ast.ASTHandle, error) {
	coefficients, err := t.base.manager.BrokenGCDTestCoefficients(proof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if len(coefficients) != len(premises) {
		return ast.ASTHandle{}, UnsupportedProofError{Proof: proof}
	}
	firstProof, firstConclusion, err := t.gcdToDivisionRule(proof, true, coefficients, premises, direct)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	secondProof, secondConclusion, err := t.gcdToDivisionRule(proof, false, coefficients, premises, direct)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	one, err := t.base.manager.MakeIntText("1")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	falsehood, err := t.base.manager.Make(OperatorFalse)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return direct.MakeFarkas(falsehood, []ast.ASTHandle{firstProof, secondProof}, []ast.ASTHandle{firstConclusion, secondConclusion}, []ast.ASTHandle{one, one})
}

func (t *FullTranslation) translateAssignBounds(proof, conclusion ast.ASTHandle, premises, premiseConclusions []ast.ASTHandle, direct *DirectProofInterpolator) (ast.ASTHandle, error) {
	var coefficients []ast.ASTHandle
	var err error
	if len(premises) == 0 {
		coefficients, err = t.base.manager.AssignBoundsCoefficientTerms(proof)
	} else {
		coefficients, err = t.base.manager.AssignBoundsRuleCoefficientTerms(proof)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	consequence := conclusion
	inputs := append([]ast.ASTHandle(nil), premiseConclusions...)
	inputProofs := append([]ast.ASTHandle(nil), premises...)
	if len(premises) == 0 {
		arguments, e := t.base.manager.Args(conclusion)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if len(arguments) != len(coefficients) || len(arguments) == 0 {
			return ast.ASTHandle{}, UnsupportedProofError{Proof: proof}
		}
		consequence = arguments[0]
		for _, argument := range arguments[1:] {
			input, e := t.base.manager.MkNot(argument)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			inputs = append(inputs, input)
			hypothesis, e := direct.MakeHypothesis(input)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			inputProofs = append(inputProofs, hypothesis)
		}
	} else if len(coefficients) != len(premises)+1 {
		return ast.ASTHandle{}, NewError("bad assign-bounds theory lemma")
	}
	if len(inputs) != len(coefficients)-1 {
		return ast.ASTHandle{}, UnsupportedProofError{Proof: proof}
	}
	sum, err := t.base.manager.SumInequalities(coefficients[1:], inputs, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sum, err = t.NormalizeInequality(sum)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	normalizedConsequence, err := t.NormalizeInequality(consequence)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	divisor, err := t.DivideInequalities(sum, normalizedConsequence)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	farkasConclusion, err := t.base.manager.SumInequalities(coefficients[1:], inputs, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	farkasConclusion, err = t.NormalizeInequality(farkasConclusion)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negative, err := t.base.manager.MkNot(farkasConclusion)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	hypothesis, err := direct.MakeHypothesis(negative)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	inputs = append(inputs, negative)
	inputProofs = append(inputProofs, hypothesis)
	one, err := t.base.manager.MakeIntText("1")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	farkasCoefficients := append([]ast.ASTHandle(nil), coefficients[1:]...)
	farkasCoefficients = append(farkasCoefficients, one)
	falsehood, err := t.base.manager.Make(OperatorFalse)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	interpolation, err := direct.MakeFarkas(falsehood, inputProofs, inputs, farkasCoefficients)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return direct.MakeCutRule(farkasConclusion, divisor, consequence, interpolation)
}

func (t *FullTranslation) translateGomoryCut(proof, conclusion ast.ASTHandle, premises, premiseConclusions []ast.ASTHandle, direct *DirectProofInterpolator) (ast.ASTHandle, error) {
	if len(premises) == 0 {
		return ast.ASTHandle{}, UnsupportedProofError{Proof: proof}
	}
	coefficients, err := t.base.manager.GomoryCutCoefficientTerms(proof)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if len(coefficients) != len(premises) {
		return ast.ASTHandle{}, NewError("bad gomory-cut theory lemma")
	}
	combined, err := t.base.manager.SumInequalities(coefficients, premiseConclusions, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	combined, err = t.NormalizeInequality(combined)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	negative, err := t.base.manager.MkNot(combined)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	hypothesis, err := direct.MakeHypothesis(negative)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	proofs := append(append([]ast.ASTHandle(nil), premises...), hypothesis)
	conclusions := append(append([]ast.ASTHandle(nil), premiseConclusions...), negative)
	one, err := t.base.manager.MakeIntText("1")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	coefficients = append(coefficients, one)
	falsehood, err := t.base.manager.Make(OperatorFalse)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	interpolation, err := direct.MakeFarkas(falsehood, proofs, conclusions, coefficients)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err := t.base.manager.Arg(combined, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	divisor, err := t.GCDOfCoefficients(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return direct.MakeCutRule(combined, divisor, conclusion, interpolation)
}

// Translate computes one direct interpolant per retained frame boundary and
// stores the precomputed results as the source's final empty-clause lemma.
func (t *FullTranslation) Translate(proof ast.ASTHandle, destination *InterpolationProof) (node ProofNode, resultErr error) {
	if err := t.open(); err != nil {
		return NullProofNode, err
	}
	if destination == nil || destination.open() != nil || destination.base != t.base || t.frames == 0 {
		return NullProofNode, ast.ErrASTHandle
	}
	if err := t.ScanSkolems(proof); err != nil {
		return NullProofNode, err
	}
	var resultRefs []ast.ASTRef
	defer func() {
		for i := len(resultRefs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, t.base.manager.ast.DelRef(&resultRefs[i]))
		}
	}()
	results := make([]ast.ASTHandle, 0, t.frames-1)
	weak, err := t.base.WeakMode()
	if err != nil {
		return NullProofNode, err
	}
	for frame := 0; frame < t.frames-1; frame++ {
		direct, e := NewDirectProofInterpolator(t.base, DownwardScopeRange(frame), weak)
		if e != nil {
			return NullProofNode, e
		}
		translated, e := t.translateMain(proof, true, direct, newTranslationMemo())
		if e == nil {
			var ref ast.ASTRef
			ref, e = direct.Interpolate(translated)
			if e == nil {
				var handle ast.ASTHandle
				handle, e = t.base.manager.ast.RefGet(ref)
				if e == nil {
					resultRefs = append(resultRefs, ref)
					results = append(results, handle)
				} else {
					e = errors.Join(e, t.base.manager.ast.DelRef(&ref))
				}
			}
		}
		e = errors.Join(e, direct.Close())
		if e != nil {
			return NullProofNode, e
		}
	}
	return destination.MakeLemma(nil, results)
}
