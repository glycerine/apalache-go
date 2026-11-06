package tab

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
)

type unfoldFinalizeAttempt struct {
	manager *ast.ASTManager
	nested  memory.ConstructionFailure
	subst   *rewriter.VarSubstitution
	temps   []ast.ASTRef
	work    *Clause
	result  *Clause
}

func (a *unfoldFinalizeAttempt) Close() error {
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
	for len(a.temps) != 0 {
		i := len(a.temps) - 1
		err := a.manager.DelRef(&a.temps[i])
		if a.temps[i] == (ast.ASTRef{}) {
			a.temps = a.temps[:i]
		}
		if err != nil {
			return err
		}
		if len(a.temps) != i {
			return ast.ErrASTPending
		}
	}
	if a.work != nil {
		if err := a.work.Close(); err != nil {
			return err
		}
		a.work = nil
	}
	if a.result != nil {
		if err := a.result.Close(); err != nil {
			return err
		}
		a.result = nil
	}
	return nil
}

type unfoldFinalizeFailure struct {
	cause, cleanupErr error
	exposed           error
	message           string
	attempt           *unfoldFinalizeAttempt
}

func (f *unfoldFinalizeFailure) Error() string { return f.message }
func (f *unfoldFinalizeFailure) Unwrap() error { return f.exposed }
func (f *unfoldFinalizeFailure) Cleanup() error {
	f.cleanupErr = f.attempt.Close()
	return f.cleanupErr
}
func (f *unfoldFinalizeFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *unfoldFinalizeFailure) OriginalCause() error { return f.cause }
func (f *unfoldFinalizeFailure) CleanupCause() error  { return f.cleanupErr }

func finishUnfoldFinalize(a *unfoldFinalizeAttempt, matched bool, cause error) (*Clause, bool, error) {
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
		return result, matched, nil
	}
	if cause == nil {
		a.result = result
		cause = cleanup
	}
	if cleanup == nil {
		return nil, false, cause
	}
	exposed := errors.Join(cause, cleanup)
	return nil, false, &unfoldFinalizeFailure{cause: cause, cleanupErr: cleanup, exposed: exposed, message: exposed.Error(), attempt: a}
}

var _ memory.ConstructionFailure = (*unfoldFinalizeFailure)(nil)
