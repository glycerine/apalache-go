package model

// Source: src/model/model.cpp:69-81; src/model/model.h:44-45.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// Evaluate is model::eval's compatibility entry point. The caller supplies
// the process services made implicit by the C++ process globals.
func (md *Model) Evaluate(expression ast.ASTHandle, out ast.ASTRef, completion bool, services ModelEvaluatorConfig) (err error) {
	if err = md.open(); err != nil {
		return err
	}
	evaluator, err := NewModelEvaluator(md.Core(), services)
	if err != nil {
		if evaluator != nil {
			err = errors.Join(err, evaluator.Close())
		}
		return err
	}
	defer func() { err = errors.Join(err, evaluator.Close()) }()
	if err = evaluator.SetModelCompletion(completion); err != nil {
		return err
	}
	return evaluator.Evaluate(expression, out)
}
