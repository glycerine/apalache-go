package qe

// Source: spacer_qe_project.cpp array_project_eqs_util store-result path.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type legacyStoreEqualityRewriteCleanupFailure struct {
	manager        *ast.ASTManager
	result         *legacyArraySelectProjection
	intermediate   ast.ASTRef
	nested         memory.ConstructionFailure
	runtime        *memory.Runtime
	cause, cleanup error
	reported       error
}

var _ memory.ConstructionFailure = (*legacyStoreEqualityRewriteCleanupFailure)(nil)

func (f *legacyStoreEqualityRewriteCleanupFailure) Error() string                 { return f.reported.Error() }
func (f *legacyStoreEqualityRewriteCleanupFailure) Unwrap() error                 { return f.reported }
func (f *legacyStoreEqualityRewriteCleanupFailure) OriginalCause() error          { return f.cause }
func (f *legacyStoreEqualityRewriteCleanupFailure) CleanupCause() error           { return f.cleanup }
func (f *legacyStoreEqualityRewriteCleanupFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *legacyStoreEqualityRewriteCleanupFailure) Cleanup() error {
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
	if f.result != nil && f.result.manager != nil {
		f.cleanup = f.result.Close()
		if f.cleanup != nil {
			return f.cleanup
		}
	}
	f.result = nil
	f.cleanup = nil
	return nil
}

// finishLegacyStoreEqualityRewrite returns result only while its caller still
// owns it. On error the result is closed or transferred to the cleanup receipt.
func finishLegacyStoreEqualityRewrite(manager *ast.ASTManager, result *legacyArraySelectProjection, intermediate ast.ASTRef, cause error) (*legacyArraySelectProjection, error) {
	cleanup := manager.DelRef(&intermediate)
	if cleanup == nil && cause == nil {
		return result, nil
	}
	if cleanup == nil && result != nil && result.manager != nil {
		cleanup = result.Close()
		if cleanup == nil {
			return nil, cause
		}
	}
	if intermediate == (ast.ASTRef{}) && (result == nil || result.manager == nil) {
		return nil, errors.Join(cause, cleanup)
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
	return nil, &legacyStoreEqualityRewriteCleanupFailure{
		manager: manager, result: result, intermediate: intermediate, nested: nested,
		runtime: manager.NumeralService().RuntimeOwner(), cause: origin, cleanup: cleanup, reported: reported,
	}
}
