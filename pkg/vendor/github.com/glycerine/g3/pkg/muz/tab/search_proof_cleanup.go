package tab

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type tabProofConverterOwner interface{ Close() error }

type tabProofAttempt struct {
	manager   *ast.ASTManager
	nested    memory.ConstructionFailure
	root      ast.ASTRef
	converter tabProofConverterOwner
	scope     *ast.ScopedProofMode
	result    ast.ASTRef
}

func (a *tabProofAttempt) Close() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if a.root != (ast.ASTRef{}) {
		if err := a.manager.DelRef(&a.root); err != nil {
			return err
		}
	}
	if a.converter != nil {
		if err := a.converter.Close(); err != nil {
			return err
		}
		a.converter = nil
	}
	if a.scope != nil {
		if err := a.scope.Close(); err != nil {
			return err
		}
		a.scope = nil
	}
	if a.result != (ast.ASTRef{}) {
		if err := a.manager.DelRef(&a.result); err != nil {
			return err
		}
	}
	return nil
}

type tabResolutionProofAttempt struct {
	manager  *ast.ASTManager
	refs     []ast.ASTRef
	sub      *proofSubstitutions
	replayed *Clause
	unfolded *unfoldedClause
}

func (a *tabResolutionProofAttempt) Close() error {
	for len(a.refs) != 0 {
		i := len(a.refs) - 1
		if err := a.manager.DelRef(&a.refs[i]); err != nil {
			return err
		}
		a.refs = a.refs[:i]
	}
	if a.sub != nil {
		if err := a.sub.Close(); err != nil {
			return err
		}
		a.sub = nil
	}
	if a.replayed != nil {
		if err := a.replayed.Close(); err != nil {
			return err
		}
		a.replayed = nil
	}
	if a.unfolded != nil {
		if err := a.unfolded.Close(); err != nil {
			return err
		}
		a.unfolded = nil
	}
	return nil
}

type tabProofCleanupFailure struct {
	cause, cleanupErr error
	root              interface{ Close() error }
	runtime           *memory.Runtime
}

func (f *tabProofCleanupFailure) Error() string                 { return errors.Join(f.cause, f.cleanupErr).Error() }
func (f *tabProofCleanupFailure) Unwrap() error                 { return errors.Join(f.cause, f.cleanupErr) }
func (f *tabProofCleanupFailure) Cleanup() error                { f.cleanupErr = f.root.Close(); return f.cleanupErr }
func (f *tabProofCleanupFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *tabProofCleanupFailure) OriginalCause() error          { return f.cause }
func (f *tabProofCleanupFailure) CleanupCause() error           { return f.cleanupErr }

func finishTabProofAttempt(a *tabProofAttempt, result ast.ASTRef, cause error) (ast.ASTRef, error) {
	if cause != nil {
		var nested memory.ConstructionFailure
		if errors.As(cause, &nested) {
			a.nested = nested
		}
	}
	cleanup := a.Close()
	if cleanup == nil && cause == nil {
		return result, nil
	}
	if cause == nil {
		cause = cleanup
	}
	if cleanup == nil && result != (ast.ASTRef{}) {
		a.result = result
		result = ast.ASTRef{}
		cleanup = a.Close()
	}
	if cleanup == nil {
		return ast.ASTRef{}, cause
	}
	if result != (ast.ASTRef{}) {
		a.result = result
	}
	return ast.ASTRef{}, &tabProofCleanupFailure{cause: cause, cleanupErr: cleanup, root: a, runtime: a.manager.NumeralService().RuntimeOwner()}
}

func finishTabResolutionProofAttempt(a *tabResolutionProofAttempt, cause error) error {
	cleanup := a.Close()
	if cleanup == nil {
		return cause
	}
	if cause == nil {
		cause = cleanup
	}
	return &tabProofCleanupFailure{cause: cause, cleanupErr: cleanup, root: a, runtime: a.manager.NumeralService().RuntimeOwner()}
}

var _ memory.ConstructionFailure = (*tabProofCleanupFailure)(nil)
