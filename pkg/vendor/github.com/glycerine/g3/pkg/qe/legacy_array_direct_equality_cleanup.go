package qe

// Source: spacer_qe_project.cpp array_project_eqs_util::mk_result direct path.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type legacyDirectEqualityRewriteCleanupFailure struct {
	manager             *ast.ASTManager
	final, intermediate ast.ASTRef
	nested              memory.ConstructionFailure
	runtime             *memory.Runtime
	cause, cleanup      error
	reported            error
}

var _ memory.ConstructionFailure = (*legacyDirectEqualityRewriteCleanupFailure)(nil)

func (f *legacyDirectEqualityRewriteCleanupFailure) Error() string                 { return f.reported.Error() }
func (f *legacyDirectEqualityRewriteCleanupFailure) Unwrap() error                 { return f.reported }
func (f *legacyDirectEqualityRewriteCleanupFailure) OriginalCause() error          { return f.cause }
func (f *legacyDirectEqualityRewriteCleanupFailure) CleanupCause() error           { return f.cleanup }
func (f *legacyDirectEqualityRewriteCleanupFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *legacyDirectEqualityRewriteCleanupFailure) Cleanup() error {
	if f.nested != nil {
		f.cleanup = f.nested.Cleanup()
		if f.cleanup != nil {
			return f.cleanup
		}
		f.nested = nil
	}
	if f.intermediate != (ast.ASTRef{}) {
		f.cleanup = f.manager.DelRef(&f.intermediate)
		if f.cleanup != nil {
			return f.cleanup
		}
	}
	if f.final != (ast.ASTRef{}) {
		f.cleanup = f.manager.DelRef(&f.final)
		if f.cleanup != nil {
			return f.cleanup
		}
	}
	f.cleanup = nil
	return nil
}

func finishLegacyDirectEqualityRewrite(manager *ast.ASTManager, final, intermediate ast.ASTRef, cause error) error {
	cleanup := manager.DelRef(&intermediate)
	if cleanup == nil && cause == nil {
		return nil
	}
	if cleanup == nil && final != (ast.ASTRef{}) {
		cleanup = manager.DelRef(&final)
		if cleanup == nil {
			return cause
		}
	}
	if final == (ast.ASTRef{}) && intermediate == (ast.ASTRef{}) {
		return errors.Join(cause, cleanup)
	}
	origin := cause
	if origin == nil {
		origin = cleanup
	}
	reported := cleanup
	if cause != nil {
		reported = errors.Join(cause, cleanup)
	}
	var nested memory.ConstructionFailure
	_ = errors.As(cause, &nested)
	return &legacyDirectEqualityRewriteCleanupFailure{
		manager: manager, final: final, intermediate: intermediate, nested: nested,
		runtime: manager.NumeralService().RuntimeOwner(), cause: origin, cleanup: cleanup, reported: reported,
	}
}
