package qe

// Source: src/muz/spacer/spacer_util.cpp:640-646 qe_project DEBUG_CODE.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (p *LegacyProjection) projectionModelHolds(md *model.ModelCore, formula ast.ASTHandle) (holds bool, resultErr error) {
	evaluator, err := newLegacyModelEvaluator(md, model.ModelEvaluatorConfig{Context: p.config.Context, Globals: p.config.Globals, Memory: p.manager.NumeralService().RuntimeOwner(), Cooperation: p.config.Cooperation, Trace: p.config.Trace})
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	if err := evaluator.SetModelCompletion(false); err != nil {
		return false, err
	}
	value, err := evaluator.EvaluateValue(formula)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&value)) }()
	handle, err := p.manager.RefGet(value)
	if err != nil {
		return false, err
	}
	return p.manager.IsTrue(handle), nil
}
