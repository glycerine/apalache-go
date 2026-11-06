package tab

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
)

type groundClauseConstructionAttempt struct {
	manager   *ast.ASTManager
	nested    memory.ConstructionFailure
	subst     *rewriter.VarSubstitution
	variables []ast.ASTRef
	temporary ast.ASTRef
	result    *groundClause
}

func (a *groundClauseConstructionAttempt) Close() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if a.subst != nil {
		if err := a.subst.Close(); err != nil {
			return err
		}
		a.subst = nil
	}
	for len(a.variables) != 0 {
		i := len(a.variables) - 1
		err := a.manager.DelRef(&a.variables[i])
		if a.variables[i] == (ast.ASTRef{}) {
			a.variables = a.variables[:i]
		}
		if err != nil {
			return err
		}
		if len(a.variables) != i {
			return ast.ErrASTPending
		}
	}
	if a.temporary != (ast.ASTRef{}) {
		if err := a.manager.DelRef(&a.temporary); err != nil {
			return err
		}
	}
	if a.result != nil {
		if err := a.result.Close(); err != nil {
			return err
		}
		a.result = nil
	}
	return nil
}

type groundClauseConstructionFailure struct {
	cause, cleanupErr, exposed error
	message                    string
	attempt                    *groundClauseConstructionAttempt
}

func (f *groundClauseConstructionFailure) Error() string { return f.message }
func (f *groundClauseConstructionFailure) Unwrap() error { return f.exposed }
func (f *groundClauseConstructionFailure) Cleanup() error {
	f.cleanupErr = f.attempt.Close()
	return f.cleanupErr
}
func (f *groundClauseConstructionFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *groundClauseConstructionFailure) OriginalCause() error { return f.cause }
func (f *groundClauseConstructionFailure) CleanupCause() error  { return f.cleanupErr }

func finishGroundClauseConstruction(a *groundClauseConstructionAttempt, cause error) (*groundClause, error) {
	if cause != nil {
		var nested memory.ConstructionFailure
		if errors.As(cause, &nested) {
			a.nested = nested
		}
	}
	result := a.result
	if cause == nil {
		a.result = nil
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
		return nil, cause
	}
	exposed := errors.Join(cause, cleanup)
	return nil, &groundClauseConstructionFailure{cause: cause, cleanupErr: cleanup, exposed: exposed, message: exposed.Error(), attempt: a}
}

var _ memory.ConstructionFailure = (*groundClauseConstructionFailure)(nil)
