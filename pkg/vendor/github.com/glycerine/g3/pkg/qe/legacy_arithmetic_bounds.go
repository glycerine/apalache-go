package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// arith_project_util::find_max.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (p *legacyArithmeticProjection) evaluateRat(evaluator *model.ModelEvaluator, expression ast.ASTHandle) (_ *big.Rat, resultErr error) {
	value, err := evaluator.EvaluateValue(expression)
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, p.syntax.manager.DelRef(&value))
	}()
	handle, err := p.syntax.manager.RefGet(value)
	if err != nil {
		return nil, err
	}
	rational, numeral, err := p.syntax.numeral(handle)
	if err != nil {
		return nil, err
	}
	if !numeral {
		return nil, ast.ErrASTDomain
	}
	return rational, nil
}

func (p *legacyArithmeticProjection) findMax(md *model.ModelCore, variable ast.ASTHandle, literals []*legacyLinearLiteral, positive bool) (_ int, resultErr error) {
	if p == nil || p.syntax == nil || md == nil || md.Manager() != p.syntax.manager {
		return -1, ast.ErrASTHandle
	}
	evaluator, err := model.NewModelEvaluator(md, model.ModelEvaluatorConfig{
		Context:     p.syntax.config.Context,
		Globals:     p.syntax.config.Globals,
		Memory:      p.syntax.manager.NumeralService().RuntimeOwner(),
		Cooperation: p.syntax.config.Cooperation,
		Trace:       p.syntax.config.Trace,
	})
	if err != nil {
		return -1, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, evaluator.Close())
	}()
	if err = evaluator.SetModelCompletion(true); err != nil {
		return -1, err
	}
	variableValue, err := p.evaluateRat(evaluator, variable)
	if err != nil {
		return -1, err
	}
	found := -1
	foundStrict := false
	var foundValue *big.Rat
	for index, literal := range literals {
		if literal == nil || literal.manager != p.syntax.manager || literal.equality || literal.coefficient.Sign() == 0 || (literal.coefficient.Sign() > 0) != positive {
			continue
		}
		term, err := p.syntax.manager.RefGet(literal.term)
		if err != nil {
			return -1, err
		}
		value, err := p.evaluateRat(evaluator, term)
		if err != nil {
			return -1, err
		}
		value.Quo(value, new(big.Rat).Abs(literal.coefficient))
		atModel := new(big.Rat).Set(value)
		if positive {
			atModel.Add(atModel, variableValue)
		} else {
			atModel.Sub(atModel, variableValue)
		}
		trueInModel := atModel.Sign() < 0 || (!literal.strict && atModel.Sign() == 0)
		if !trueInModel {
			continue
		}
		if found < 0 || value.Cmp(foundValue) > 0 || (value.Cmp(foundValue) == 0 && !foundStrict && literal.strict) {
			found = index
			foundValue = new(big.Rat).Set(value)
			foundStrict = literal.strict
		}
	}
	if found < 0 {
		return -1, ast.ErrASTDomain
	}
	return found, nil
}
