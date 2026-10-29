package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// arith_project_util::project, real-arithmetic paths.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

type legacyLiteralReplacement struct {
	literal     ast.ASTHandle
	replacement ast.ASTRef
}

type legacyRealProjection struct {
	manager      *ast.ASTManager
	substitution ast.ASTRef
	divisibility ast.ASTRef
	replacements []legacyLiteralReplacement
}

func (r *legacyRealProjection) Close() (result error) {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	for index := len(r.replacements) - 1; index >= 0; index-- {
		result = errors.Join(result, r.manager.DelRef(&r.replacements[index].replacement))
	}
	r.replacements = nil
	if r.substitution != (ast.ASTRef{}) {
		result = errors.Join(result, r.manager.DelRef(&r.substitution))
	}
	if r.divisibility != (ast.ASTRef{}) {
		result = errors.Join(result, r.manager.DelRef(&r.divisibility))
	}
	if result == nil {
		r.manager = nil
	}
	return result
}

func (p *legacyArithmeticProjection) appendLegacyReplacement(result *legacyRealProjection, literal ast.ASTHandle, replacement ast.ASTRef) {
	result.replacements = append(result.replacements, legacyLiteralReplacement{literal: literal, replacement: replacement})
}

func (p *legacyArithmeticProjection) constantLegacy(value bool) (ast.ASTRef, error) {
	handle, err := p.syntax.manager.MkFalse()
	if value {
		handle, err = p.syntax.manager.MkTrue()
	}
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.syntax.manager.NewRef(handle)
}

func (p *legacyArithmeticProjection) projectReal(md *model.ModelCore, variable ast.ASTHandle, inputs []ast.ASTHandle) (_ *legacyRealProjection, projected bool, resultErr error) {
	if p == nil || p.syntax == nil || md == nil || md.Manager() != p.syntax.manager {
		return nil, false, ast.ErrASTHandle
	}
	integer, arithmetic, err := p.syntax.arithmetic(variable)
	if err != nil {
		return nil, false, err
	}
	if !arithmetic || integer {
		return nil, false, ast.ErrASTDomain
	}
	result := &legacyRealProjection{manager: p.syntax.manager}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
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
			if err = result.Close(); err != nil {
				return nil, false, err
			}
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
		cx, err := p.multiplyLegacy(literal.coefficient, variable)
		if err != nil {
			return nil, false, err
		}
		cxt, err := p.syntax.manager.MkArithmeticAdd(cx, term)
		if err != nil {
			return nil, false, err
		}
		value, err := p.evaluateRat(evaluator, cxt)
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
					newTerm, err := p.syntax.manager.NewRef(negated)
					if err != nil {
						return nil, false, err
					}
					if err = p.syntax.manager.DelRef(&literal.term); err != nil {
						_ = p.syntax.manager.DelRef(&newTerm)
						return nil, false, err
					}
					literal.term = newTerm
					value.Neg(value)
				}
				literal.strict = true
				literal.disequality = false
			}
			trueInModel := value.Sign() < 0 || (!literal.strict && value.Sign() == 0)
			if trueInModel {
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
		return result, true, nil
	}
	if equalityIndex >= 0 {
		literal := literals[equalityIndex]
		term, err := p.syntax.manager.RefGet(literal.term)
		if err != nil {
			return nil, false, err
		}
		substitution, err := p.multiplyLegacy(new(big.Rat).Neg(new(big.Rat).Inv(literal.coefficient)), term)
		if err != nil {
			return nil, false, err
		}
		result.substitution, err = p.rewriteLegacy(substitution)
		return result, err == nil, err
	}
	if positive == 0 || negative == 0 {
		for index, literal := range literals {
			value := !(literal.equality || (positive == 0 && literal.coefficient.Sign() > 0) || (negative == 0 && literal.coefficient.Sign() < 0))
			replacement, err := p.constantLegacy(value)
			if err != nil {
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
	for index, literal := range literals {
		var replacement ast.ASTRef
		switch {
		case index == selected:
			replacement, err = p.constantLegacy(true)
		case literal.equality && !literals[selected].strict:
			replacement, err = p.equalityAtBound(literal, literals[selected])
		case literal.equality:
			replacement, err = p.constantLegacy(false)
		case (literal.coefficient.Sign() > 0) == usePositive:
			replacement, err = p.sameBound(literal, literals[selected])
		default:
			replacement, err = p.oppositeBound(literal, literals[selected])
		}
		if err != nil {
			return nil, false, err
		}
		p.appendLegacyReplacement(result, originals[index], replacement)
	}
	return result, true, nil
}
