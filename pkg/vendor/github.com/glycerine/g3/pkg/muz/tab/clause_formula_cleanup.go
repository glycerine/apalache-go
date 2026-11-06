package tab

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type clauseFormulaAttempt struct {
	manager *ast.ASTManager
	nested  memory.ConstructionFailure
	result  ast.ASTRef
}

func (a *clauseFormulaAttempt) Close() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if a.result != (ast.ASTRef{}) {
		if err := a.manager.DelRef(&a.result); err != nil {
			return err
		}
	}
	return nil
}

type clauseFormulaFailure struct {
	cause, cleanupErr, exposed error
	message                    string
	attempt                    *clauseFormulaAttempt
}

func (f *clauseFormulaFailure) Error() string { return f.message }
func (f *clauseFormulaFailure) Unwrap() error { return f.exposed }
func (f *clauseFormulaFailure) Cleanup() error {
	f.cleanupErr = f.attempt.Close()
	return f.cleanupErr
}
func (f *clauseFormulaFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *clauseFormulaFailure) OriginalCause() error { return f.cause }
func (f *clauseFormulaFailure) CleanupCause() error  { return f.cleanupErr }

func finishClauseFormula(a *clauseFormulaAttempt, result ast.ASTRef, cause error) (ast.ASTRef, error) {
	if cause != nil {
		var nested memory.ConstructionFailure
		if errors.As(cause, &nested) {
			a.nested = nested
		}
	}
	if cause == nil {
		a.result = ast.ASTRef{}
	}
	cleanup := a.Close()
	if cleanup == nil && cause == nil {
		return result, nil
	}
	if cause == nil {
		a.result = result
		cause = cleanup
	}
	if cleanup == nil {
		return ast.ASTRef{}, cause
	}
	exposed := errors.Join(cause, cleanup)
	return ast.ASTRef{}, &clauseFormulaFailure{cause: cause, cleanupErr: cleanup, exposed: exposed, message: exposed.Error(), attempt: a}
}

var _ memory.ConstructionFailure = (*clauseFormulaFailure)(nil)
