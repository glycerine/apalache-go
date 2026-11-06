package interp

// Source checkpoint: src/interp/iz3translate.cpp arithmetic certificate
// reconstruction for equality propagation and unnamed arithmetic lemmas.

import (
	"errors"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *FullTranslation) sortSum(term ast.ASTHandle) (ast.ASTHandle, error) {
	op, err := t.base.manager.Operator(term)
	if err != nil || op != OperatorPlus {
		return term, err
	}
	arguments, err := t.base.manager.Args(term)
	if err != nil || len(arguments) < 2 {
		return term, err
	}
	type indexedTerm struct {
		term ast.ASTHandle
		id   uint32
	}
	indexed := make([]indexedTerm, len(arguments))
	for i, argument := range arguments {
		id, e := t.base.manager.ASTID(argument)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		indexed[i] = indexedTerm{term: argument, id: id}
	}
	sort.SliceStable(indexed, func(i, j int) bool { return indexed[i].id < indexed[j].id })
	for i := range indexed {
		arguments[i] = indexed[i].term
	}
	return t.base.manager.Make(OperatorPlus, arguments...)
}

func (t *FullTranslation) sumVector(term ast.ASTHandle, coefficients *[]*big.Rat, variables *[]ast.ASTHandle) error {
	op, err := t.base.manager.Operator(term)
	if err != nil {
		return err
	}
	if op == OperatorPlus {
		arguments, e := t.base.manager.Args(term)
		if e != nil {
			return e
		}
		for _, argument := range arguments {
			if e = t.sumVector(argument, coefficients, variables); e != nil {
				return e
			}
		}
		return nil
	}
	coefficient, err := t.base.manager.Coefficient(term)
	if err != nil {
		return err
	}
	variable, err := t.base.manager.LinearVariable(term)
	if err != nil {
		return err
	}
	*coefficients = append(*coefficients, coefficient)
	*variables = append(*variables, variable)
	return nil
}

func (t *FullTranslation) replaceSummands(term ast.ASTHandle, replacements map[ast.ASTHandle]ast.ASTHandle) (ast.ASTHandle, error) {
	op, err := t.base.manager.Operator(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if op == OperatorPlus {
		arguments, e := t.base.manager.Args(term)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		for i := range arguments {
			arguments[i], e = t.replaceSummands(arguments[i], replacements)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		return t.base.manager.Make(OperatorPlus, arguments...)
	}
	if op == OperatorTimes {
		coefficient, e := t.base.manager.Arg(term, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		value, e := t.base.manager.Arg(term, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		value, e = t.replaceSummands(value, replacements)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return t.base.manager.Make(OperatorTimes, coefficient, value)
	}
	if replacement, found := replacements[term]; found {
		return replacement, nil
	}
	sort, err := t.base.manager.Type(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	replacement, err := t.base.manager.FreshConstant("@s", sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	replacements[term] = replacement
	return replacement, nil
}

func (t *FullTranslation) reallyNormalizeInequality(inequality ast.ASTHandle) (ast.ASTHandle, error) {
	normalized, err := t.NormalizeInequality(inequality)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err := t.base.manager.Arg(normalized, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err = t.sortSum(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := t.base.manager.Arg(normalized, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	op, err := t.base.manager.Operator(normalized)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return t.base.manager.Make(op, left, right)
}

func (t *FullTranslation) painfullyNormalizeInequality(inequality ast.ASTHandle, replacements map[ast.ASTHandle]ast.ASTHandle) (ast.ASTHandle, error) {
	normalized, err := t.NormalizeInequality(inequality)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err := t.base.manager.Arg(normalized, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err = t.replaceSummands(left, replacements)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err = t.sortSum(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := t.base.manager.Arg(normalized, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	op, err := t.base.manager.Operator(normalized)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return t.base.manager.Make(op, left, right)
}

func (t *FullTranslation) reconstructFarkasFromProof(sourcePremises, translatedPremises []ast.ASTHandle, conclusion ast.ASTHandle, painful bool, direct *DirectProofInterpolator) (ast.ASTHandle, bool, error) {
	replacements := make(map[ast.ASTHandle]ast.ASTHandle)
	normalizedToProof := make(map[ast.ASTHandle]ast.ASTHandle, len(sourcePremises))
	normalizedToConclusion := make(map[ast.ASTHandle]ast.ASTHandle, len(sourcePremises))
	normalized := make([]ast.ASTHandle, 0, len(sourcePremises)+1)
	for i, source := range sourcePremises {
		premiseConclusion, err := t.base.manager.Conclusion(source)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		var value ast.ASTHandle
		if painful {
			value, err = t.painfullyNormalizeInequality(premiseConclusion, replacements)
		} else {
			value, err = t.reallyNormalizeInequality(premiseConclusion)
		}
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		normalized = append(normalized, value)
		normalizedToProof[value] = translatedPremises[i]
		normalizedToConclusion[value] = premiseConclusion
	}
	negated, err := t.base.manager.MkNot(conclusion)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	var normalizedNegation ast.ASTHandle
	if painful {
		normalizedNegation, err = t.painfullyNormalizeInequality(negated, replacements)
	} else {
		normalizedNegation, err = t.reallyNormalizeInequality(negated)
	}
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	normalized = append(normalized, normalizedNegation)
	sat, err := t.base.IsSAT(normalized, nil)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	defer func() { _ = sat.Close() }()
	if sat.Satisfiable() {
		return ast.ASTHandle{}, false, ProofError{}
	}
	newProof, err := t.base.manager.ast.RefGet(sat.Proof)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	rule, err := t.base.manager.ProofRule(newProof)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	count, err := t.base.manager.NumPremises(newProof)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	var coefficients []ast.ASTHandle
	if rule == ast.ASTPrTheoryLemma {
		theory, e := t.base.manager.TheoryLemmaTheory(newProof)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		kind, e := t.base.manager.TheoryLemmaKind(newProof)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if theory != ArithTheory || kind != FarkasKind {
			return ast.ASTHandle{}, false, nil
		}
		coefficients, err = t.base.manager.FarkasCoefficientTerms(newProof)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
	} else if rule == ast.ASTPrUnitResolution && count == 2 {
		one, e := t.base.manager.MakeIntText("1")
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		coefficients = make([]ast.ASTHandle, count)
		for i := range coefficients {
			coefficients[i] = one
		}
	} else {
		return ast.ASTHandle{}, false, nil
	}
	if len(coefficients) != int(count) {
		return ast.ASTHandle{}, false, nil
	}
	proofs := make([]ast.ASTHandle, 0, count)
	conclusions := make([]ast.ASTHandle, 0, count)
	for i := uint32(0); i < count; i++ {
		premise, e := t.base.manager.Premise(newProof, i)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		premiseConclusion, e := t.base.manager.Conclusion(premise)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		premiseConclusion, e = t.reallyNormalizeInequality(premiseConclusion)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if translated, found := normalizedToProof[premiseConclusion]; found {
			proofs = append(proofs, translated)
			conclusions = append(conclusions, normalizedToConclusion[premiseConclusion])
		} else if premiseConclusion == normalizedNegation {
			hypothesis, e := direct.MakeHypothesis(negated)
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			proofs = append(proofs, hypothesis)
			conclusions = append(conclusions, negated)
		} else {
			return ast.ASTHandle{}, false, nil
		}
	}
	falsehood, err := t.base.manager.Make(OperatorFalse)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	result, err := direct.MakeFarkas(falsehood, proofs, conclusions, coefficients)
	return result, err == nil, err
}

func rationalDenominatorLCM(values []*big.Rat) *big.Int {
	result := big.NewInt(1)
	for _, value := range values {
		gcd := new(big.Int).GCD(nil, nil, result, value.Denom())
		result.Mul(result, value.Denom())
		result.Quo(result, gcd)
	}
	return result
}

func (t *FullTranslation) reconstructFarkasWithDual(sourcePremises, translatedPremises []ast.ASTHandle, conclusion ast.ASTHandle, direct *DirectProofInterpolator) (result ast.ASTHandle, resultErr error) {
	replacements := make(map[ast.ASTHandle]ast.ASTHandle)
	normalized := make([]ast.ASTHandle, 0, len(sourcePremises)+1)
	for _, source := range sourcePremises {
		premiseConclusion, err := t.base.manager.Conclusion(source)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		value, err := t.painfullyNormalizeInequality(premiseConclusion, replacements)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if op, e := t.base.manager.Operator(value); e != nil {
			return ast.ASTHandle{}, e
		} else if op == OperatorLT {
			left, e := t.base.manager.Arg(value, 0)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			right, e := t.base.manager.Arg(value, 1)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			one, e := t.base.manager.MakeIntText("1")
			if e != nil {
				return ast.ASTHandle{}, e
			}
			right, e = t.base.manager.Make(OperatorSub, right, one)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			right, e = t.ownedSimplify(right)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			value, e = t.base.manager.Make(OperatorLE, left, right)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		normalized = append(normalized, value)
	}
	negated, err := t.base.manager.MkNot(conclusion)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	normalizedNegation, err := t.painfullyNormalizeInequality(negated, replacements)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	normalized = append(normalized, normalizedNegation)
	realSort, err := t.base.manager.RealType()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zero, err := t.base.manager.MakeReal(new(big.Rat))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	coefficients := make([]ast.ASTHandle, 0, len(normalized))
	dual := make(map[ast.ASTHandle]ast.ASTHandle)
	var variablesSeen []ast.ASTHandle
	rhs := zero
	for _, inequality := range normalized {
		coefficient, e := t.base.manager.FreshConstant("@c", realSort)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		coefficients = append(coefficients, coefficient)
		left, e := t.base.manager.Arg(inequality, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		var values []*big.Rat
		var variables []ast.ASTHandle
		if e = t.sumVector(left, &values, &variables); e != nil {
			return ast.ASTHandle{}, e
		}
		for i, variable := range variables {
			current, found := dual[variable]
			if !found {
				current = zero
				variablesSeen = append(variablesSeen, variable)
			}
			factor, e := t.base.manager.MakeReal(values[i])
			if e != nil {
				return ast.ASTHandle{}, e
			}
			product, e := t.base.manager.Make(OperatorTimes, factor, coefficient)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			current, e = t.base.manager.Make(OperatorPlus, current, product)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			dual[variable] = current
		}
		right, e := t.base.manager.Arg(inequality, 1)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		product, e := t.base.manager.Make(OperatorTimes, coefficient, right)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		rhs, e = t.base.manager.Make(OperatorPlus, rhs, product)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	constraints := make([]ast.ASTHandle, 0, len(variablesSeen)+len(coefficients)+1)
	for _, variable := range variablesSeen {
		equality, e := t.base.manager.Make(OperatorEqual, dual[variable], zero)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		constraints = append(constraints, equality)
	}
	bound, err := t.base.manager.Make(OperatorLE, rhs, zero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	constraints = append(constraints, bound)
	for _, coefficient := range coefficients[:len(coefficients)-1] {
		nonnegative, e := t.base.manager.Make(OperatorGE, coefficient, zero)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		constraints = append(constraints, nonnegative)
	}
	one, err := t.base.manager.MakeReal(big.NewRat(1, 1))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	last, err := t.base.manager.Make(OperatorEqual, coefficients[len(coefficients)-1], one)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	constraints = append(constraints, last)
	for _, coefficient := range coefficients[:len(coefficients)-1] {
		zeroConstraint, e := t.base.manager.Make(OperatorEqual, coefficient, zero)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		constraints = append(constraints, zeroConstraint)
		check, e := t.base.IsSAT(constraints, nil)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		unsatisfiable := !check.Satisfiable()
		if closeErr := check.Close(); closeErr != nil {
			return ast.ASTHandle{}, closeErr
		}
		if unsatisfiable {
			constraints = constraints[:len(constraints)-1]
		}
	}
	model, err := t.base.IsSAT(constraints, coefficients)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, model.Close()) }()
	if !model.Satisfiable() || len(model.Values) != len(coefficients) {
		return ast.ASTHandle{}, ProofError{}
	}
	values := make([]*big.Rat, len(model.Values))
	for i := range model.Values {
		value, e := t.base.manager.ast.RefGet(model.Values[i])
		if e != nil {
			return ast.ASTHandle{}, e
		}
		var numeral bool
		values[i], numeral, e = t.base.manager.Numeral(value)
		if e != nil || !numeral || values[i] == nil {
			if e != nil {
				return ast.ASTHandle{}, e
			}
			return ast.ASTHandle{}, ProofError{}
		}
	}
	lcm := rationalDenominatorLCM(values)
	farkasCoefficients := make([]ast.ASTHandle, 0, len(values))
	proofs := make([]ast.ASTHandle, 0, len(values))
	conclusions := make([]ast.ASTHandle, 0, len(values))
	for i := range sourcePremises {
		scaled := new(big.Rat).Mul(values[i], new(big.Rat).SetInt(lcm))
		if scaled.Sign() == 0 {
			continue
		}
		coefficient, e := t.base.manager.MakeInt(scaled)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		premiseConclusion, e := t.base.manager.Conclusion(sourcePremises[i])
		if e != nil {
			return ast.ASTHandle{}, e
		}
		farkasCoefficients = append(farkasCoefficients, coefficient)
		proofs = append(proofs, translatedPremises[i])
		conclusions = append(conclusions, premiseConclusion)
	}
	lastCoefficient, err := t.base.manager.MakeInt(new(big.Rat).SetInt(lcm))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	hypothesis, err := direct.MakeHypothesis(negated)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	farkasCoefficients = append(farkasCoefficients, lastCoefficient)
	proofs = append(proofs, hypothesis)
	conclusions = append(conclusions, negated)
	falsehood, err := t.base.manager.Make(OperatorFalse)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return direct.MakeFarkas(falsehood, proofs, conclusions, farkasCoefficients)
}

func (t *FullTranslation) reconstructFarkas(sourcePremises, translatedPremises []ast.ASTHandle, conclusion ast.ASTHandle, direct *DirectProofInterpolator) (ast.ASTHandle, error) {
	if len(sourcePremises) != len(translatedPremises) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if result, found, err := t.reconstructFarkasFromProof(sourcePremises, translatedPremises, conclusion, false, direct); err != nil {
		return ast.ASTHandle{}, err
	} else if found {
		return result, nil
	}
	if result, found, err := t.reconstructFarkasFromProof(sourcePremises, translatedPremises, conclusion, true, direct); err != nil {
		return ast.ASTHandle{}, err
	} else if found {
		return result, nil
	}
	return t.reconstructFarkasWithDual(sourcePremises, translatedPremises, conclusion, direct)
}

func (t *FullTranslation) translateEqualityPropagation(conclusion ast.ASTHandle, sourcePremises, translatedPremises []ast.ASTHandle, direct *DirectProofInterpolator) (ast.ASTHandle, error) {
	left, err := t.base.manager.Arg(conclusion, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := t.base.manager.Arg(conclusion, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	inequalities := make([]ast.ASTHandle, 2)
	proofs := make([]ast.ASTHandle, 2)
	inequalities[0], err = t.base.manager.Make(OperatorLE, left, right)
	if err == nil {
		inequalities[1], err = t.base.manager.Make(OperatorGE, left, right)
	}
	for i := range inequalities {
		if err == nil {
			proofs[i], err = t.reconstructFarkas(sourcePremises, translatedPremises, inequalities[i], direct)
		}
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := direct.MakeLeqToEq(left, right, inequalities[0], inequalities[1])
	for i := range inequalities {
		if err == nil {
			result, err = direct.MakeResolution(inequalities[i], nil, result, proofs[i])
		}
	}
	return result, err
}
