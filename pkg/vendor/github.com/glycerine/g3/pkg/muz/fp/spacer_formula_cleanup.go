package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
)

// A sliced formula conversion consumes its source reference, including on
// failure. A refused cleanup must keep that copied owner reachable for retry.
type slicedFormulaAttempt struct {
	manager        *ast.ASTManager
	source, result ast.ASTRef
	substitute     *astrewriter.VarSubstitution
	bindings       []ast.ASTRef
}

type slicedFormulaCleanupFailure struct {
	cause      error
	cleanupErr error
	attempt    *slicedFormulaAttempt
}

func (f *slicedFormulaCleanupFailure) Error() string { return f.cause.Error() }
func (f *slicedFormulaCleanupFailure) Unwrap() error { return f.cause }
func (f *slicedFormulaCleanupFailure) Cleanup() error {
	f.cleanupErr = f.attempt.close()
	return f.cleanupErr
}
func (f *slicedFormulaCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *slicedFormulaCleanupFailure) OriginalCause() error { return f.cause }
func (f *slicedFormulaCleanupFailure) CleanupCause() error  { return f.cleanupErr }
func (a *slicedFormulaAttempt) closeTemporary() error {
	if a.substitute != nil {
		if err := a.substitute.Close(); err != nil {
			return err
		}
		a.substitute = nil
	}
	for len(a.bindings) != 0 {
		last := len(a.bindings) - 1
		if a.bindings[last] != (ast.ASTRef{}) {
			if err := a.manager.DelRef(&a.bindings[last]); err != nil {
				return err
			}
		}
		a.bindings = a.bindings[:last]
	}
	if a.source != (ast.ASTRef{}) {
		return a.manager.DelRef(&a.source)
	}
	return nil
}
func (a *slicedFormulaAttempt) close() error {
	if err := a.closeTemporary(); err != nil {
		return err
	}
	if a.result != (ast.ASTRef{}) {
		return a.manager.DelRef(&a.result)
	}
	return nil
}
func (a *slicedFormulaAttempt) finish(cause error) (ast.ASTRef, error) {
	cause = errors.Join(cause, a.closeTemporary())
	if cause == nil {
		result := a.result
		a.result = ast.ASTRef{}
		return result, nil
	}
	if cleanup := a.close(); cleanup != nil {
		return ast.ASTRef{}, &slicedFormulaCleanupFailure{cause: errors.Join(cause, cleanup), cleanupErr: cleanup, attempt: a}
	}
	return ast.ASTRef{}, cause
}
