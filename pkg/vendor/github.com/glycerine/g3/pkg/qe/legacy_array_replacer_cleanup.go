package qe

// Source: src/muz/spacer/spacer_qe_project.cpp local expr_safe_replace owners.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
)

type legacyArrayReplacerCleanupFailure struct {
	manager  *ast.ASTManager
	replacer *rewriter.ExprSafeReplace
	nested   memory.ConstructionFailure
	runtime  *memory.Runtime
	cause    error
	cleanup  error
}

var _ memory.ConstructionFailure = (*legacyArrayReplacerCleanupFailure)(nil)

func (f *legacyArrayReplacerCleanupFailure) Error() string {
	return errors.Join(f.cause, f.cleanup).Error()
}
func (f *legacyArrayReplacerCleanupFailure) Unwrap() error                 { return errors.Join(f.cause, f.cleanup) }
func (f *legacyArrayReplacerCleanupFailure) OriginalCause() error          { return f.cause }
func (f *legacyArrayReplacerCleanupFailure) CleanupCause() error           { return f.cleanup }
func (f *legacyArrayReplacerCleanupFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *legacyArrayReplacerCleanupFailure) Cleanup() error {
	if f.nested != nil {
		f.cleanup = f.nested.Cleanup()
		if f.cleanup != nil {
			return f.cleanup
		}
		f.nested = nil
	}
	if f.replacer != nil {
		f.cleanup = f.replacer.Close()
		if f.cleanup != nil {
			return f.cleanup
		}
		f.replacer = nil
	}
	f.cleanup = nil
	return nil
}

func closeFailedLegacyArrayReplacer(manager *ast.ASTManager, replacer *rewriter.ExprSafeReplace, cause error) error {
	cleanup := replacer.Close()
	if cleanup == nil {
		return cause
	}
	origin := cause
	if origin == nil {
		origin = cleanup
	}
	var nested memory.ConstructionFailure
	_ = errors.As(cause, &nested)
	return &legacyArrayReplacerCleanupFailure{
		manager: manager, replacer: replacer, nested: nested,
		runtime: manager.NumeralService().RuntimeOwner(), cause: origin, cleanup: cleanup,
	}
}
