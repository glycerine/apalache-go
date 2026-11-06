package tab

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/memory"
)

type tautologyCheckAttempt struct {
	manager *ast.ASTManager
	nested  memory.ConstructionFailure
	solver  solver.Solver
	pushed  bool
	target  uint32
	formula ast.ASTRef
}

func (a *tautologyCheckAttempt) Close() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if a.pushed {
		level, err := a.solver.ScopeLevel()
		if err != nil {
			return err
		}
		if level == a.target {
			a.pushed = false
		} else if level != a.target+1 {
			return ast.ErrASTPending
		}
		if a.pushed {
			if err = a.solver.Pop(1); err != nil {
				return err
			}
		}
		a.pushed = false
	}
	if a.formula != (ast.ASTRef{}) {
		if err := a.manager.DelRef(&a.formula); err != nil {
			return err
		}
	}
	return nil
}

type tautologyCheckFailure struct {
	cause, cleanupErr, exposed error
	message                    string
	attempt                    *tautologyCheckAttempt
}

func (f *tautologyCheckFailure) Error() string { return f.message }
func (f *tautologyCheckFailure) Unwrap() error { return f.exposed }
func (f *tautologyCheckFailure) Cleanup() error {
	f.cleanupErr = f.attempt.Close()
	return f.cleanupErr
}
func (f *tautologyCheckFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *tautologyCheckFailure) OriginalCause() error { return f.cause }
func (f *tautologyCheckFailure) CleanupCause() error  { return f.cleanupErr }

func finishTautologyCheck(a *tautologyCheckAttempt, result bool, cause error) (bool, error) {
	if cause != nil {
		var nested memory.ConstructionFailure
		if errors.As(cause, &nested) {
			a.nested = nested
		}
	}
	cleanup := a.Close()
	if cleanup == nil {
		return result, cause
	}
	if cause == nil {
		cause = cleanup
	}
	exposed := errors.Join(cause, cleanup)
	return false, &tautologyCheckFailure{cause: cause, cleanupErr: cleanup, exposed: exposed, message: exposed.Error(), attempt: a}
}

var _ memory.ConstructionFailure = (*tautologyCheckFailure)(nil)
