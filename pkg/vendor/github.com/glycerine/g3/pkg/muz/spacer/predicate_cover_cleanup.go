package spacer

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
)

// coverExportAttempt owns get_cover_delta's temporary conversion roots until
// cleanup completes, including when Go's native AST admission guard refuses.
type coverExportAttempt struct {
	manager         *ast.ASTManager
	runtime         *memory.Runtime
	formula, result ast.ASTRef
	substitution    ast.ASTExprSubstitution
	replacer        *rewriter.DefaultExprReplacer
}

func (a *coverExportAttempt) closeTemps() error {
	if a.formula != (ast.ASTRef{}) {
		if err := a.manager.DelRef(&a.formula); err != nil {
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
func (a *coverExportAttempt) close() error {
	if a.result != (ast.ASTRef{}) {
		if err := a.manager.DelRef(&a.result); err != nil {
			return err
		}
	}
	return a.closeTemps()
}
func (a *coverExportAttempt) finish(cause error) (ast.ASTRef, error) {
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
	return ast.ASTRef{}, &coverExportFailure{attempt: a, original: cause, reported: reported, text: reported.Error(), cleanupErr: cleanup}
}

type coverExportFailure struct {
	attempt                        *coverExportAttempt
	original, reported, cleanupErr error
	text                           string
}

func (f *coverExportFailure) Error() string                 { return f.text }
func (f *coverExportFailure) Unwrap() error                 { return f.reported }
func (f *coverExportFailure) OriginalCause() error          { return f.original }
func (f *coverExportFailure) CleanupCause() error           { return f.cleanupErr }
func (f *coverExportFailure) RuntimeOwner() *memory.Runtime { return f.attempt.runtime }
func (f *coverExportFailure) Cleanup() error                { f.cleanupErr = f.attempt.close(); return f.cleanupErr }

var _ memory.ConstructionFailure = (*coverExportFailure)(nil)
