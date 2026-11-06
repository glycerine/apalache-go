package opt

// Source: src/opt/maxres.cpp is_true/is_false model helpers.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (e *maxResEngine) modelBoolean(md *model.ModelCore, expression ast.ASTHandle) (value bool, resultErr error) {
	if formula, found := e.proxies[expression]; found {
		return e.modelBoolean(md, formula)
	}
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
	ref, err := evaluator.EvaluateValue(expression)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, e.solver.manager.DelRef(&ref)) }()
	handle, err := e.solver.manager.RefGet(ref)
	if err != nil {
		return false, err
	}
	return e.solver.manager.IsTrue(handle), nil
}
