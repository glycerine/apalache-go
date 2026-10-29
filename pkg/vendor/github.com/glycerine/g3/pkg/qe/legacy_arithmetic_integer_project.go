package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// arith_project_util::project, integer virtual substitution paths.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func legacyIntegerMod(value, modulus *big.Rat) (*big.Rat, error) {
	if value == nil || modulus == nil || !value.IsInt() || !modulus.IsInt() || modulus.Sign() == 0 {
		return nil, ast.ErrASTDomain
	}
	m := new(big.Int).Abs(modulus.Num())
	return new(big.Rat).SetInt(new(big.Int).Mod(value.Num(), m)), nil
}

func (p *legacyArithmeticProjection) integerDivisibility(term ast.ASTHandle, divisor *big.Rat) (ast.ASTRef, error) {
	modulus, err := p.syntax.manager.ArithmeticNumeralRat(divisor, true)
	if err != nil {
		return ast.ASTRef{}, err
	}
	modular, err := p.syntax.manager.MkFamilyApp(p.syntax.family, 13, nil, []ast.ASTHandle{term, modulus})
	if err != nil {
		return ast.ASTRef{}, err
	}
	zero, err := p.syntax.manager.ArithmeticIntegerNumeralText("0")
	if err != nil {
		return ast.ASTRef{}, err
	}
	equality, err := p.syntax.manager.MkEq(modular, zero)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.syntax.manager.NewRef(equality)
}

func (p *legacyArithmeticProjection) projectInteger(md *model.ModelCore, variable ast.ASTHandle, inputs []ast.ASTHandle) (result *legacyRealProjection, projected bool, resultErr error) {
	if p == nil || p.syntax == nil || md == nil || md.Manager() != p.syntax.manager {
		return nil, false, ast.ErrASTHandle
	}
	defer func() {
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
			projected = false
		}
	}()
	integer, arithmetic, err := p.syntax.arithmetic(variable)
	if err != nil {
		return nil, false, err
	}
	if !arithmetic || !integer {
		return nil, false, ast.ErrASTDomain
	}
	evaluator, err := model.NewModelEvaluator(md, model.ModelEvaluatorConfig{
		Context: p.syntax.config.Context, Globals: p.syntax.config.Globals,
		Memory: p.syntax.manager.NumeralService().RuntimeOwner(), Cooperation: p.syntax.config.Cooperation, Trace: p.syntax.config.Trace,
	})
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	if err = evaluator.SetModelCompletion(true); err != nil {
		return nil, false, err
	}
	variableValue, err := p.evaluateRat(evaluator, variable)
	if err != nil {
		return nil, false, err
	}
	literals := make([]*legacyLinearLiteral, 0, len(inputs))
	originals := make([]ast.ASTHandle, 0, len(inputs))
	defer func() { resultErr = errors.Join(resultErr, closeLegacyLinearLiterals(literals...)) }()
	equalityIndex := -1
	positive, negative := 0, 0
	for _, input := range inputs {
		dependent, err := p.contains(input, variable)
		if err != nil {
			return nil, false, err
		}
		if !dependent {
			continue
		}
		literal, err := p.linearLiteral(variable, input)
		if err != nil {
			return nil, false, err
		}
		if literal == nil {
			return nil, false, nil
		}
		if literal.coefficient.Sign() == 0 {
			if err = literal.Close(); err != nil {
				return nil, false, err
			}
			continue
		}
		term, err := p.syntax.manager.RefGet(literal.term)
		if err != nil {
			return nil, false, err
		}
		coefficientTerm, err := p.multiplyLegacy(literal.coefficient, variable)
		if err != nil {
			return nil, false, err
		}
		sum, err := p.syntax.manager.MkArithmeticAdd(coefficientTerm, term)
		if err != nil {
			return nil, false, err
		}
		value, err := p.evaluateRat(evaluator, sum)
		if err != nil {
			return nil, false, err
		}
		if literal.equality {
			if equalityIndex < 0 && value.Sign() == 0 {
				equalityIndex = len(literals)
			}
		} else {
			if literal.disequality {
				if value.Sign() > 0 {
					literal.coefficient.Neg(literal.coefficient)
					negated, err := p.multiplyLegacy(big.NewRat(-1, 1), term)
					if err != nil {
						return nil, false, err
					}
					if err = p.replaceLegacyTerm(literal, negated); err != nil {
						return nil, false, err
					}
					value.Neg(value)
				}
				literal.strict = true
				literal.disequality = false
			}
			if literal.divisor.Sign() == 0 && (value.Sign() < 0 || (!literal.strict && value.Sign() == 0)) {
				if literal.coefficient.Sign() > 0 {
					positive++
				} else {
					negative++
				}
			}
		}
		literals = append(literals, literal)
		originals = append(originals, input)
	}
	if len(literals) == 0 {
		return &legacyRealProjection{manager: p.syntax.manager}, true, nil
	}
	lcmCoefficients, lcmDivisors, err := p.normalizeIntegers(literals)
	if err != nil {
		return nil, false, err
	}
	attachDivisibility := func(result *legacyRealProjection, xTerm ast.ASTHandle) error {
		if lcmCoefficients.Cmp(big.NewRat(1, 1)) == 0 {
			return nil
		}
		constraint, err := p.integerDivisibility(xTerm, lcmCoefficients)
		if err == nil {
			result.divisibility = constraint
		}
		return err
	}
	if equalityIndex >= 0 {
		literal := literals[equalityIndex]
		xTerm, err := p.syntax.manager.RefGet(literal.term)
		if err != nil {
			return nil, false, err
		}
		if literal.coefficient.Sign() > 0 {
			xTerm, err = p.syntax.manager.MkArithmeticNeg(xTerm)
			if err != nil {
				return nil, false, err
			}
		}
		rewritten, err := p.rewriteLegacy(xTerm)
		if err != nil {
			return nil, false, err
		}
		xTerm, err = p.syntax.manager.RefGet(rewritten)
		if err != nil {
			_ = p.syntax.manager.DelRef(&rewritten)
			return nil, false, err
		}
		result, err = p.substituteIntegerLiterals(literals, originals, xTerm, equalityIndex)
		if err == nil {
			err = attachDivisibility(result, xTerm)
		}
		err = errors.Join(err, p.syntax.manager.DelRef(&rewritten))
		if err != nil {
			if result != nil {
				err = errors.Join(err, result.Close())
			}
			return nil, false, err
		}
		return result, true, nil
	}
	if positive == 0 || negative == 0 {
		xValue := new(big.Rat).Mul(lcmCoefficients, variableValue)
		xValue, err = legacyIntegerMod(xValue, lcmDivisors)
		if err != nil {
			return nil, false, err
		}
		xTerm, err := p.syntax.manager.ArithmeticNumeralRat(xValue, true)
		if err != nil {
			return nil, false, err
		}
		result = &legacyRealProjection{manager: p.syntax.manager}
		for index, literal := range literals {
			var replacement ast.ASTRef
			if literal.divisor.Sign() != 0 {
				term, err := p.syntax.manager.RefGet(literal.term)
				if err != nil {
					_ = result.Close()
					return nil, false, err
				}
				if literal.coefficient.Sign() > 0 {
					term, err = p.syntax.manager.MkArithmeticAdd(term, xTerm)
				} else {
					negated, negateErr := p.syntax.manager.MkArithmeticNeg(xTerm)
					if negateErr == nil {
						term, err = p.syntax.manager.MkArithmeticAdd(term, negated)
					} else {
						err = negateErr
					}
				}
				if err == nil {
					var rewritten ast.ASTRef
					rewritten, err = p.rewriteLegacy(term)
					if err == nil {
						term, err = p.syntax.manager.RefGet(rewritten)
					}
					if err == nil {
						replacement, err = p.integerDivisibility(term, literal.divisor)
					}
					if rewritten != (ast.ASTRef{}) {
						err = errors.Join(err, p.syntax.manager.DelRef(&rewritten))
					}
				}
			} else {
				value := !(literal.equality || (positive == 0 && literal.coefficient.Sign() > 0) || (negative == 0 && literal.coefficient.Sign() < 0))
				replacement, err = p.constantLegacy(value)
			}
			if err != nil {
				err = errors.Join(err, result.Close())
				return nil, false, err
			}
			p.appendLegacyReplacement(result, originals[index], replacement)
		}
		return result, true, nil
	}
	usePositive := positive < negative
	selected, err := p.findMax(md, variable, literals, usePositive)
	if err != nil {
		return nil, false, err
	}
	selectedTerm, err := p.syntax.manager.RefGet(literals[selected].term)
	if err != nil {
		return nil, false, err
	}
	coefficientTerm, err := p.multiplyLegacy(literals[selected].coefficient, variable)
	if err != nil {
		return nil, false, err
	}
	sum, err := p.syntax.manager.MkArithmeticAdd(coefficientTerm, selectedTerm)
	if err != nil {
		return nil, false, err
	}
	value, err := p.evaluateRat(evaluator, sum)
	if err != nil {
		return nil, false, err
	}
	offset := new(big.Rat).Abs(value)
	if literals[selected].strict {
		offset.Sub(offset, big.NewRat(1, 1))
	}
	offset, err = legacyIntegerMod(offset, lcmDivisors)
	if err != nil {
		return nil, false, err
	}
	offsetTerm, err := p.syntax.manager.ArithmeticNumeralRat(offset, true)
	if err != nil {
		return nil, false, err
	}
	xTerm, err := p.syntax.manager.MkArithmeticAdd(selectedTerm, offsetTerm)
	if err == nil && literals[selected].strict {
		one, oneErr := p.syntax.manager.ArithmeticIntegerNumeralText("1")
		if oneErr == nil {
			xTerm, err = p.syntax.manager.MkArithmeticAdd(xTerm, one)
		} else {
			err = oneErr
		}
	}
	if err == nil && literals[selected].coefficient.Sign() > 0 {
		xTerm, err = p.syntax.manager.MkArithmeticNeg(xTerm)
	}
	if err != nil {
		return nil, false, err
	}
	rewritten, err := p.rewriteLegacy(xTerm)
	if err != nil {
		return nil, false, err
	}
	xTerm, err = p.syntax.manager.RefGet(rewritten)
	if err != nil {
		_ = p.syntax.manager.DelRef(&rewritten)
		return nil, false, err
	}
	result, err = p.substituteIntegerLiterals(literals, originals, xTerm, selected)
	if err == nil {
		err = attachDivisibility(result, xTerm)
	}
	err = errors.Join(err, p.syntax.manager.DelRef(&rewritten))
	if err != nil {
		if result != nil {
			err = errors.Join(err, result.Close())
		}
		return nil, false, err
	}
	return result, true, nil
}
