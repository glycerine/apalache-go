package tab

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type clauseConstructionAttempt struct {
	manager *ast.ASTManager
	nested  memory.ConstructionFailure
	clause  *Clause
}

func (a *clauseConstructionAttempt) Close() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if a.clause != nil {
		if err := a.clause.Close(); err != nil {
			return err
		}
		a.clause = nil
	}
	return nil
}

type clauseConstructionFailure struct {
	cause, cleanupErr, exposed error
	message                    string
	attempt                    *clauseConstructionAttempt
}

func (f *clauseConstructionFailure) Error() string { return f.message }
func (f *clauseConstructionFailure) Unwrap() error { return f.exposed }
func (f *clauseConstructionFailure) Cleanup() error {
	f.cleanupErr = f.attempt.Close()
	return f.cleanupErr
}
func (f *clauseConstructionFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *clauseConstructionFailure) OriginalCause() error { return f.cause }
func (f *clauseConstructionFailure) CleanupCause() error  { return f.cleanupErr }

func finishClauseConstruction(a *clauseConstructionAttempt, cause error) (*Clause, error) {
	if cause != nil {
		var nested memory.ConstructionFailure
		if errors.As(cause, &nested) {
			a.nested = nested
		}
	}
	result := a.clause
	if cause == nil {
		a.clause = nil
	}
	cleanup := a.Close()
	if cleanup == nil && cause == nil {
		return result, nil
	}
	if cause == nil {
		a.clause = result
		cause = cleanup
	}
	if cleanup == nil {
		return nil, cause
	}
	exposed := errors.Join(cause, cleanup)
	return nil, &clauseConstructionFailure{cause: cause, cleanupErr: cleanup, exposed: exposed, message: exposed.Error(), attempt: a}
}

var _ memory.ConstructionFailure = (*clauseConstructionFailure)(nil)
