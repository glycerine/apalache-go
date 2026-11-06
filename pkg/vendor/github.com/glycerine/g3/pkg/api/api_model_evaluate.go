package api

// Source: src/api/api_model.cpp:160-173.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// Evaluate evaluates expression in this retained model. The returned ASTRef
// owns its result independently of the model and must be released by DelRef.
func (m *APIModel) Evaluate(expression ast.ASTHandle, completion bool) (result ast.ASTRef, resultErr error) {
	if err := m.open(); err != nil {
		return result, err
	}
	derived := m.core.DerivedModel()
	if derived == nil {
		return result, ast.ErrASTDomain
	}
	config, err := m.owner.command.ModelEvaluatorConfig()
	if err != nil {
		return result, err
	}
	result, err = m.core.Manager().NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	if err = derived.Evaluate(expression, result, completion, config); err != nil {
		return ast.ASTRef{}, errorsJoinDelRef(err, m.core.Manager(), &result)
	}
	return result, nil
}

func errorsJoinDelRef(cause error, manager *ast.ASTManager, value *ast.ASTRef) error {
	if cleanup := manager.DelRef(value); cleanup != nil {
		return errors.Join(cause, cleanup)
	}
	return cause
}
