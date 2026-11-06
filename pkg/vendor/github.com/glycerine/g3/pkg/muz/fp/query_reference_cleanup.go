package fp

// Query rule/background copies own temporary expression vectors. Preserve the
// remaining roots when a native reference release refuses during unwinding.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type queryReferenceCleanupFailure struct {
	manager        *ast.ASTManager
	runtime        *memory.Runtime
	references     []ast.ASTRef
	cause, cleanup error
}

func (f *queryReferenceCleanupFailure) Error() string                 { return f.cause.Error() }
func (f *queryReferenceCleanupFailure) Unwrap() error                 { return f.cause }
func (f *queryReferenceCleanupFailure) OriginalCause() error          { return f.cause }
func (f *queryReferenceCleanupFailure) CleanupCause() error           { return f.cleanup }
func (f *queryReferenceCleanupFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *queryReferenceCleanupFailure) Cleanup() error {
	f.cleanup = releaseQueryReferences(f.manager, f.references)
	if f.cleanup == nil {
		f.references = nil
	}
	return f.cleanup
}

func releaseQueryReferences(manager *ast.ASTManager, references []ast.ASTRef) (result error) {
	for i := len(references) - 1; i >= 0; i-- {
		if references[i] != (ast.ASTRef{}) {
			result = errors.Join(result, manager.DelRef(&references[i]))
		}
	}
	return result
}

func closeASTRefs(manager *ast.ASTManager, references []ast.ASTRef) error {
	if err := releaseQueryReferences(manager, references); err != nil {
		return &queryReferenceCleanupFailure{manager: manager, runtime: manager.NumeralService().RuntimeOwner(), references: references, cause: err, cleanup: err}
	}
	return nil
}
