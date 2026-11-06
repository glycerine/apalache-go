package qe

// Source: spacer_qe_project.cpp array_project local result/current owners.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type legacyArrayFormulaCleanupFailure struct {
	manager        *ast.ASTManager
	current        ast.ASTRef
	result         *legacyArrayProjectionResult
	nested         memory.ConstructionFailure
	runtime        *memory.Runtime
	cause, cleanup error
	reported       error
}

var _ memory.ConstructionFailure = (*legacyArrayFormulaCleanupFailure)(nil)

func (f *legacyArrayFormulaCleanupFailure) Error() string                 { return f.reported.Error() }
func (f *legacyArrayFormulaCleanupFailure) Unwrap() error                 { return f.reported }
func (f *legacyArrayFormulaCleanupFailure) OriginalCause() error          { return f.cause }
func (f *legacyArrayFormulaCleanupFailure) CleanupCause() error           { return f.cleanup }
func (f *legacyArrayFormulaCleanupFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *legacyArrayFormulaCleanupFailure) Cleanup() error {
	if f.nested != nil {
		f.cleanup = f.nested.Cleanup()
		if f.cleanup != nil {
			return f.cleanup
		}
		f.nested = nil
	}
	if f.current != (ast.ASTRef{}) {
		f.cleanup = f.manager.DelRef(&f.current)
		if f.cleanup != nil {
			return f.cleanup
		}
	}
	if f.result != nil {
		f.cleanup = f.result.Close()
		if f.cleanup != nil {
			return f.cleanup
		}
		f.result = nil
	}
	f.cleanup = nil
	return nil
}

// finishLegacyArrayFormula releases the lexical current owner before
// publishing a successful result. On an operation error it additionally
// destroys the unpublished result in source lifetime order. A refused first
// release transports both still-live owners to one retryable receipt.
func finishLegacyArrayFormula(manager *ast.ASTManager, current ast.ASTRef, result *legacyArrayProjectionResult, cause error) error {
	cleanup := manager.DelRef(&current)
	if cleanup == nil && cause == nil {
		return nil
	}
	if cleanup == nil && result != nil {
		cleanup = result.Close()
		if cleanup == nil {
			return cause
		}
		if result.manager == nil {
			result = nil
		}
	}
	if current == (ast.ASTRef{}) && result == nil {
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
	return &legacyArrayFormulaCleanupFailure{
		manager: manager, current: current, result: result, nested: nested, runtime: manager.NumeralService().RuntimeOwner(),
		cause: origin, cleanup: cleanup, reported: reported,
	}
}
