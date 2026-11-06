package interp

// Source checkpoint: src/interp/iz3base.cpp is_sat.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/messages"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// BaseSATResult owns the proof or evaluated model values returned by IsSAT.
type BaseSATResult struct {
	manager *ast.ASTManager
	Status  lbool.LBool
	Proof   ast.ASTRef
	Values  []ast.ASTRef
	closed  bool
}

func (r *BaseSATResult) Satisfiable() bool { return r != nil && r.Status != lbool.False }

func (r *BaseSATResult) Close() (result error) {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	for i := len(r.Values) - 1; i >= 0; i-- {
		result = errors.Join(result, r.manager.DelRef(&r.Values[i]))
	}
	r.Values = nil
	if r.Proof != (ast.ASTRef{}) {
		result = errors.Join(result, r.manager.DelRef(&r.Proof))
	}
	r.closed = true
	return result
}

func (b *Base) IsSAT(formulas, variables []ast.ASTHandle) (result *BaseSATResult, resultErr error) {
	if err := b.open(); err != nil {
		return nil, err
	}
	if b.solver == nil {
		return nil, NewError("interpolation base has no solver factory")
	}
	var parameters uparams.ParamsRef
	var parameterPointer *uparams.ParamsRef
	if b.manager.context != nil {
		var err error
		parameters, err = b.manager.context.NewValue()
		if err != nil {
			return nil, err
		}
		parameterPointer = &parameters
		defer func() { resultErr = errors.Join(resultErr, b.manager.context.Del(&parameters)) }()
	}
	s, err := b.solver.New(b.manager.ast, parameterPointer, true, true, true, "")
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, s.Close()) }()
	for _, formula := range formulas {
		if err = s.Assert(formula); err != nil {
			return nil, err
		}
	}
	status, err := s.CheckSat(nil)
	if err != nil {
		return nil, err
	}
	if canceled, e := b.manager.ast.Canceled(); e != nil {
		return nil, e
	} else if canceled {
		return nil, NewError(messages.Canceled)
	}
	result = &BaseSATResult{manager: b.manager.ast, Status: status}
	defer func() {
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	if status == lbool.False {
		proof, e := s.Proof()
		if e != nil {
			return nil, e
		}
		result.Proof, e = b.manager.ast.NewRef(proof)
		if e != nil {
			return nil, e
		}
		return result, nil
	}
	if len(variables) == 0 {
		return result, nil
	}
	model, err := s.AcquireModel()
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, NewError("interpolation cannot proceed without a model")
	}
	defer func() { resultErr = errors.Join(resultErr, model.DecRef()) }()
	if b.evaluator == nil {
		return nil, NewError("interpolation base has no model evaluator")
	}
	evaluator, err := b.evaluator(model)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	for _, variable := range variables {
		value, e := evaluator.EvaluateValue(variable)
		if e != nil {
			return nil, e
		}
		result.Values = append(result.Values, value)
	}
	return result, nil
}
