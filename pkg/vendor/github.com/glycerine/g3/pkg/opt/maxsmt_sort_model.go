package opt

// Source: src/opt/sortmax.cpp update_assignment/is_true.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (e *sortMaxEngine) updateResult(result *ArithmeticResult) error {
	cost, satisfied, assignment, err := e.solver.evaluateSoftDetailed(e.objective, result.Model)
	if err != nil {
		return err
	}
	setSoftObjectiveValue(result, e.index, cost, satisfied, assignment)
	return nil
}

func (e *sortMaxEngine) countSatisfiedInputs(md *model.ModelCore) (count int, resultErr error) {
	evaluator, err := model.NewModelEvaluator(md, e.solver.config.ModelEvaluator)
	if err != nil {
		if evaluator != nil {
			err = errors.Join(err, evaluator.Close())
		}
		return 0, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	if err = evaluator.SetModelCompletion(true); err != nil {
		return 0, err
	}
	for _, input := range e.inputs {
		value, evalErr := evaluator.EvaluateValue(input)
		if evalErr != nil {
			return 0, evalErr
		}
		h, evalErr := e.solver.manager.RefGet(value)
		isTrue := evalErr == nil && e.solver.manager.IsTrue(h)
		evalErr = errors.Join(evalErr, e.solver.manager.DelRef(&value))
		if evalErr != nil {
			return 0, evalErr
		}
		if isTrue {
			count++
		}
	}
	return count, nil
}

func (e *sortMaxEngine) outputIsTrue(md *model.ModelCore, output ast.ASTHandle) (isTrue bool, resultErr error) {
	evaluator, err := model.NewModelEvaluator(md, e.solver.config.ModelEvaluator)
	if err != nil {
		if evaluator != nil {
			err = errors.Join(err, evaluator.Close())
		}
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	if err = evaluator.SetModelCompletion(true); err != nil {
		return false, err
	}
	value, err := evaluator.EvaluateValue(output)
	if err != nil {
		return false, err
	}
	h, err := e.solver.manager.RefGet(value)
	isTrue = err == nil && e.solver.manager.IsTrue(h)
	return isTrue, errors.Join(err, e.solver.manager.DelRef(&value))
}

func (e *sortMaxEngine) setBounds(result *ArithmeticResult, lower, upper *big.Rat) {
	value := &result.Values[e.index]
	value.SoftCost.Set(lower)
	value.SoftLower.Set(lower)
	value.SoftUpper.Set(upper)
	value.SoftBoundsValid = true
	total := new(big.Rat).Set(&e.objective.softOffset)
	for _, weight := range e.objective.softWeights {
		total.Add(total, weight)
	}
	value.SoftSatisfied.Sub(total, lower)
	value.Soft = true
}
