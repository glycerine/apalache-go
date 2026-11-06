package interp

// Source checkpoint: src/interp/iz3translate.cpp arithmetic normalization
// helpers used by retained proof-rule translation.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *FullTranslation) ownedSimplify(term ast.ASTHandle) (ast.ASTHandle, error) {
	ref, err := t.base.manager.ReallySimplify(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := t.base.manager.ast.RefGet(ref)
	if err != nil {
		return ast.ASTHandle{}, errors.Join(err, t.base.manager.ast.DelRef(&ref))
	}
	t.owned = append(t.owned, ref)
	return result, nil
}

func (t *FullTranslation) ownedIntegerDivision(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	ref, err := t.base.manager.MakeIDiv(left, right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := t.base.manager.ast.RefGet(ref)
	if err != nil {
		return ast.ASTHandle{}, errors.Join(err, t.base.manager.ast.DelRef(&ref))
	}
	t.owned = append(t.owned, ref)
	return result, nil
}

func (t *FullTranslation) linearCoefficients(term ast.ASTHandle) ([]*big.Rat, error) {
	op, err := t.base.manager.Operator(term)
	if err != nil {
		return nil, err
	}
	terms := []ast.ASTHandle{term}
	if op == OperatorPlus {
		terms, err = t.base.manager.Args(term)
		if err != nil {
			return nil, err
		}
	}
	result := make([]*big.Rat, 0, len(terms))
	for _, value := range terms {
		coefficient, e := t.base.manager.Coefficient(value)
		if e != nil {
			return nil, e
		}
		result = append(result, coefficient)
	}
	return result, nil
}

func rationalGCD(first, second *big.Rat) *big.Rat {
	a := new(big.Int).Abs(new(big.Int).Set(first.Num()))
	b := new(big.Int).Abs(new(big.Int).Set(second.Num()))
	numerator := new(big.Int).GCD(nil, nil, a, b)
	denominatorGCD := new(big.Int).GCD(nil, nil, first.Denom(), second.Denom())
	denominator := new(big.Int).Mul(first.Denom(), second.Denom())
	denominator.Quo(denominator, denominatorGCD)
	return new(big.Rat).SetFrac(numerator, denominator)
}

func (t *FullTranslation) GCDOfCoefficients(term ast.ASTHandle) (ast.ASTHandle, error) {
	if err := t.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	coefficients, err := t.linearCoefficients(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	value := new(big.Rat).SetInt64(1)
	if len(coefficients) != 0 {
		value.Abs(coefficients[0])
		for _, coefficient := range coefficients[1:] {
			value = rationalGCD(value, coefficient)
		}
	}
	return t.base.manager.MakeInt(value)
}

func (t *FullTranslation) BoundedVariable(inequality ast.ASTHandle) (variable ast.ASTHandle, lower bool, err error) {
	normalized, err := t.NormalizeInequality(inequality)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	left, err := t.base.manager.Arg(normalized, 0)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	op, err := t.base.manager.Operator(left)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if op == OperatorUninterpreted {
		return left, false, nil
	}
	if op != OperatorTimes {
		return ast.ASTHandle{}, false, ProofError{}
	}
	coefficientTerm, err := t.base.manager.Arg(left, 0)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	coefficient, numeral, err := t.base.manager.Numeral(coefficientTerm)
	if err != nil || !numeral {
		return ast.ASTHandle{}, false, ProofError{}
	}
	switch coefficient.Cmp(new(big.Rat).SetInt64(1)) {
	case 0:
		lower = false
	default:
		if coefficient.Cmp(new(big.Rat).SetInt64(-1)) != 0 {
			return ast.ASTHandle{}, false, ProofError{}
		}
		lower = true
	}
	variable, err = t.base.manager.Arg(left, 1)
	return variable, lower, err
}

func (t *FullTranslation) TermCoefficient(inequality, variable ast.ASTHandle) (*big.Rat, error) {
	normalized, err := t.NormalizeInequality(inequality)
	if err != nil {
		return nil, err
	}
	left, err := t.base.manager.Arg(normalized, 0)
	if err != nil {
		return nil, err
	}
	terms := []ast.ASTHandle{left}
	if op, e := t.base.manager.Operator(left); e != nil {
		return nil, e
	} else if op == OperatorPlus {
		terms, err = t.base.manager.Args(left)
		if err != nil {
			return nil, err
		}
	}
	for _, term := range terms {
		candidate, e := t.base.manager.LinearVariable(term)
		if e != nil {
			return nil, e
		}
		if candidate == variable {
			return t.base.manager.Coefficient(term)
		}
	}
	return new(big.Rat), nil
}

func (t *FullTranslation) firstCoefficient(term ast.ASTHandle) (*big.Rat, ast.ASTHandle, error) {
	terms := []ast.ASTHandle{term}
	if op, err := t.base.manager.Operator(term); err != nil {
		return nil, ast.ASTHandle{}, err
	} else if op == OperatorPlus {
		terms, err = t.base.manager.Args(term)
		if err != nil {
			return nil, ast.ASTHandle{}, err
		}
	}
	var best ast.ASTHandle
	var bestID uint64 = ^uint64(0)
	coefficient := new(big.Rat)
	for _, value := range terms {
		if _, numeral, err := t.base.manager.Numeral(value); err != nil {
			return nil, ast.ASTHandle{}, err
		} else if numeral {
			continue
		}
		variable, err := t.base.manager.LinearVariable(value)
		if err != nil {
			return nil, ast.ASTHandle{}, err
		}
		id, err := t.base.manager.ASTID(variable)
		if err != nil {
			return nil, ast.ASTHandle{}, err
		}
		if uint64(id) < bestID {
			best, bestID = variable, uint64(id)
			coefficient, err = t.base.manager.Coefficient(value)
			if err != nil {
				return nil, ast.ASTHandle{}, err
			}
		}
	}
	return coefficient, best, nil
}

func (t *FullTranslation) DivideInequalities(first, second ast.ASTHandle) (ast.ASTHandle, error) {
	firstLeft, err := t.base.manager.Arg(first, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	secondLeft, err := t.base.manager.Arg(second, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	firstCoefficient, firstVariable, err := t.firstCoefficient(firstLeft)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	secondCoefficient, secondVariable, err := t.firstCoefficient(secondLeft)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if firstCoefficient.Sign() == 0 || secondCoefficient.Sign() == 0 || firstVariable != secondVariable {
		return ast.ASTHandle{}, ProofError{}
	}
	ratio := new(big.Rat).Quo(firstCoefficient, secondCoefficient)
	if !ratio.IsInt() {
		return ast.ASTHandle{}, ProofError{}
	}
	return t.base.manager.MakeInt(ratio)
}

func (t *FullTranslation) RHSNormalizeInequality(inequality ast.ASTHandle) (ast.ASTHandle, error) {
	if err := t.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	zero, err := t.base.manager.MakeIntText("0")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	one, err := t.base.manager.MakeIntText("1")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := t.base.manager.Make(OperatorLE, zero, zero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err = t.base.manager.LinearCombination(result, one, inequality, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return t.base.manager.SimplifyInequality(result)
}

func (t *FullTranslation) CheckFarkas(premises []ast.ASTHandle, conclusion ast.ASTHandle) (bool, error) {
	if err := t.open(); err != nil {
		return false, err
	}
	one, err := t.base.manager.MakeIntText("1")
	if err != nil {
		return false, err
	}
	negativeOne, err := t.base.manager.MakeIntText("-1")
	if err != nil {
		return false, err
	}
	coefficients := make([]ast.ASTHandle, len(premises)+1)
	inequalities := make([]ast.ASTHandle, len(premises)+1)
	for i := range premises {
		coefficients[i] = one
		inequalities[i] = premises[i]
	}
	coefficients[len(premises)] = negativeOne
	inequalities[len(premises)] = conclusion
	combined, err := t.base.manager.SumInequalities(coefficients, inequalities, false)
	if err != nil {
		return false, err
	}
	right, err := t.base.manager.Arg(combined, 1)
	if err != nil {
		return false, err
	}
	right, err = t.ownedSimplify(right)
	if err != nil {
		return false, err
	}
	value, numeral, err := t.base.manager.Numeral(right)
	return numeral && value.Sign() == 0, err
}

func (t *FullTranslation) NormalizeInequality(inequality ast.ASTHandle) (ast.ASTHandle, error) {
	if err := t.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := t.RHSNormalizeInequality(inequality)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	op, err := t.base.manager.Operator(result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err := t.base.manager.Arg(result, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := t.base.manager.Arg(result, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	_, numeral, err := t.base.manager.Numeral(right)
	if err != nil || numeral {
		return result, err
	}
	zero, err := t.base.manager.MakeIntText("0")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightOp, err := t.base.manager.Operator(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if rightOp == OperatorPlus {
		arguments, e := t.base.manager.Args(right)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		constant := zero
		firstNonconstant := 0
		if len(arguments) != 0 {
			if _, matched, e := t.base.manager.Numeral(arguments[0]); e != nil {
				return ast.ASTHandle{}, e
			} else if matched {
				constant = arguments[0]
				firstNonconstant = 1
			}
		}
		if firstNonconstant < len(arguments) {
			nonconstant, e := t.base.manager.Make(OperatorPlus, arguments[firstNonconstant:]...)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			left, e = t.base.manager.Make(OperatorSub, left, nonconstant)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		right = constant
	} else {
		positive, e := t.base.manager.Make(OperatorPlus, right)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		left, e = t.base.manager.Make(OperatorSub, left, positive)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		right = zero
	}
	left, err = t.ownedSimplify(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err = t.ownedSimplify(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return t.base.manager.Make(op, left, right)
}
