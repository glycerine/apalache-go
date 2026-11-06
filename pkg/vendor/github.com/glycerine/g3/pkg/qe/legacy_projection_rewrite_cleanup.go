package qe

// Source: spacer_qe_project.cpp qe_project's theory-rewriter temporary.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

// legacyProjectionRewriteCleanupFailure owns the rewriter's final temporary
// reference when the rewritten formula was copied into the projection result
// but native cleanup could not release the extra local owner.
type legacyProjectionRewriteCleanupFailure struct {
	manager        *ast.ASTManager
	reference      ast.ASTRef
	runtime        *memory.Runtime
	cause, cleanup error
	reported       error
}

var _ memory.ConstructionFailure = (*legacyProjectionRewriteCleanupFailure)(nil)

func (f *legacyProjectionRewriteCleanupFailure) Error() string                 { return f.reported.Error() }
func (f *legacyProjectionRewriteCleanupFailure) Unwrap() error                 { return f.reported }
func (f *legacyProjectionRewriteCleanupFailure) OriginalCause() error          { return f.cause }
func (f *legacyProjectionRewriteCleanupFailure) CleanupCause() error           { return f.cleanup }
func (f *legacyProjectionRewriteCleanupFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *legacyProjectionRewriteCleanupFailure) Cleanup() error {
	if f.reference == (ast.ASTRef{}) {
		f.cleanup = nil
		return nil
	}
	f.cleanup = f.manager.DelRef(&f.reference)
	return f.cleanup
}

func closeLegacyProjectionRewriteResult(manager *ast.ASTManager, reference ast.ASTRef, cause error) error {
	cleanup := manager.DelRef(&reference)
	if cleanup == nil {
		return cause
	}
	// A consumed release has no owner left to retry.
	if reference == (ast.ASTRef{}) {
		return errors.Join(cause, cleanup)
	}
	origin := cause
	if origin == nil {
		origin = cleanup
	}
	return &legacyProjectionRewriteCleanupFailure{
		manager: manager, reference: reference, runtime: manager.NumeralService().RuntimeOwner(),
		cause: origin, cleanup: cleanup, reported: errors.Join(origin, cleanup),
	}
}
