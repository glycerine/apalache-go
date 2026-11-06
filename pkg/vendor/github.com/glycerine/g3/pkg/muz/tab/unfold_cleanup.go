package tab

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/substitution"
	"github.com/glycerine/g3/pkg/util/memory"
)

type unfoldConstructionAttempt struct {
	manager      *ast.ASTManager
	nested       memory.ConstructionFailure
	temporary    []ast.ASTRef
	right, left  ast.ASTRef
	unifier      *substitution.Unifier
	substitution *substitution.Substitution
	result       *unfoldedClause
}

func (a *unfoldConstructionAttempt) Close() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	for len(a.temporary) != 0 {
		i := len(a.temporary) - 1
		err := a.manager.DelRef(&a.temporary[i])
		if a.temporary[i] == (ast.ASTRef{}) {
			a.temporary = a.temporary[:i]
		}
		if err != nil {
			return err
		}
		if len(a.temporary) != i {
			return ast.ErrASTPending
		}
	}
	for _, ref := range []*ast.ASTRef{&a.right, &a.left} {
		if *ref != (ast.ASTRef{}) {
			if err := a.manager.DelRef(ref); err != nil {
				return err
			}
		}
	}
	if a.unifier != nil {
		if err := a.unifier.Close(); err != nil {
			return err
		}
		a.unifier = nil
	}
	if a.substitution != nil {
		if err := a.substitution.Close(); err != nil {
			return err
		}
		a.substitution = nil
	}
	if a.result != nil {
		if err := a.result.Close(); err != nil {
			return err
		}
		a.result = nil
	}
	return nil
}

type unfoldConstructionFailure struct {
	cause, cleanupErr error
	message           string
	attempt           *unfoldConstructionAttempt
}

func (f *unfoldConstructionFailure) Error() string { return f.message }
func (f *unfoldConstructionFailure) Unwrap() error { return f.cause }
func (f *unfoldConstructionFailure) Cleanup() error {
	f.cleanupErr = f.attempt.Close()
	return f.cleanupErr
}
func (f *unfoldConstructionFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *unfoldConstructionFailure) OriginalCause() error { return f.cause }
func (f *unfoldConstructionFailure) CleanupCause() error  { return f.cleanupErr }

func finishUnfoldConstruction(a *unfoldConstructionAttempt, matched bool, cause error) (*unfoldedClause, bool, error) {
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
	}
	if cause == nil {
		cause = cleanup
	}
	if cleanup == nil {
		return nil, false, cause
	}
	return nil, false, &unfoldConstructionFailure{cause: cause, cleanupErr: cleanup, message: cause.Error(), attempt: a}
}

var _ memory.ConstructionFailure = (*unfoldConstructionFailure)(nil)
