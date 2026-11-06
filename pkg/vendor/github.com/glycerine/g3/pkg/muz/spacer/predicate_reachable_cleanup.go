package spacer

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
)

// reachableExportAttempt owns get_reachable's temporary conversion roots until
// cleanup completes, including when Go's native AST admission guard refuses.
type reachableExportAttempt struct {
	manager      *ast.ASTManager
	runtime      *memory.Runtime
	result       ast.ASTRef
	rewritten    []ast.ASTRef
	substitution ast.ASTExprSubstitution
	replacer     *rewriter.SimplifyingExprReplacer
}

func (a *reachableExportAttempt) closeTemps() error {
	for len(a.rewritten) > 0 {
		i := len(a.rewritten) - 1
		err := a.manager.DelRef(&a.rewritten[i])
		if a.rewritten[i] == (ast.ASTRef{}) {
			a.rewritten = a.rewritten[:i]
		}
		if err != nil {
			return err
		}
	}
	if a.replacer != nil {
		if err := a.replacer.Close(); err != nil {
			return err
		}
		a.replacer = nil
	}
	if a.substitution != (ast.ASTExprSubstitution{}) {
		if err := a.manager.DelExprSubstitution(&a.substitution); err != nil {
			return err
		}
	}
	return nil
}
func (a *reachableExportAttempt) close() error {
	if a.result != (ast.ASTRef{}) {
		if err := a.manager.DelRef(&a.result); err != nil {
			return err
		}
	}
	return a.closeTemps()
}
func (a *reachableExportAttempt) finish(cause error) (ast.ASTRef, error) {
	if cause == nil {
		cause = a.closeTemps()
	}
	if cause == nil {
		result := a.result
		a.result = ast.ASTRef{}
		return result, nil
	}
	cleanup := a.close()
	if cleanup == nil {
		return ast.ASTRef{}, cause
	}
	reported := cause
	if !errors.Is(cause, cleanup) {
		reported = errors.Join(cause, cleanup)
	}
	return ast.ASTRef{}, &reachableExportFailure{attempt: a, original: cause, reported: reported, text: reported.Error(), cleanupErr: cleanup}
}

type reachableExportFailure struct {
	attempt                        *reachableExportAttempt
	original, reported, cleanupErr error
	text                           string
}

func (f *reachableExportFailure) Error() string                 { return f.text }
func (f *reachableExportFailure) Unwrap() error                 { return f.reported }
func (f *reachableExportFailure) OriginalCause() error          { return f.original }
func (f *reachableExportFailure) CleanupCause() error           { return f.cleanupErr }
func (f *reachableExportFailure) RuntimeOwner() *memory.Runtime { return f.attempt.runtime }
func (f *reachableExportFailure) Cleanup() error {
	f.cleanupErr = f.attempt.close()
	return f.cleanupErr
}

var _ memory.ConstructionFailure = (*reachableExportFailure)(nil)
