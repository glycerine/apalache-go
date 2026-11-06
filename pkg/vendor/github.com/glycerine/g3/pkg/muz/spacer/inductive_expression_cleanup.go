package spacer

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

// inductiveExprExportAttempt owns to_expr's model and formula builder until
// their cleanup succeeds; failed cleanup cannot publish an answer.
type inductiveExprExportAttempt struct {
	manager *ast.ASTManager
	runtime *memory.Runtime
	result  ast.ASTRef
	model   *model.Model
	builder *model.Model2ExprBuilder
}

func (a *inductiveExprExportAttempt) closeTemps() error {
	// Check admission through the copied result before entering composite
	// teardown, which may consume individual model-builder roots.
	if a.result != (ast.ASTRef{}) {
		if _, err := a.manager.RefGet(a.result); err != nil {
			return err
		}
	}
	if a.builder != nil {
		if err := a.builder.Close(); err != nil {
			return err
		}
		a.builder = nil
	}
	if a.model != nil {
		if err := a.model.Close(); err != nil {
			return err
		}
		a.model = nil
	}
	return nil
}
func (a *inductiveExprExportAttempt) close() error {
	if a.result != (ast.ASTRef{}) {
		if err := a.manager.DelRef(&a.result); err != nil {
			return err
		}
	}
	return a.closeTemps()
}
func (a *inductiveExprExportAttempt) finish(cause error) (ast.ASTRef, error) {
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
	return ast.ASTRef{}, &inductiveExprExportFailure{attempt: a, original: cause, reported: reported, text: reported.Error(), cleanupErr: cleanup}
}

type inductiveExprExportFailure struct {
	attempt                        *inductiveExprExportAttempt
	original, reported, cleanupErr error
	text                           string
}

func (f *inductiveExprExportFailure) Error() string                 { return f.text }
func (f *inductiveExprExportFailure) Unwrap() error                 { return f.reported }
func (f *inductiveExprExportFailure) OriginalCause() error          { return f.original }
func (f *inductiveExprExportFailure) CleanupCause() error           { return f.cleanupErr }
func (f *inductiveExprExportFailure) RuntimeOwner() *memory.Runtime { return f.attempt.runtime }
func (f *inductiveExprExportFailure) Cleanup() error {
	f.cleanupErr = f.attempt.close()
	return f.cleanupErr
}

var _ memory.ConstructionFailure = (*inductiveExprExportFailure)(nil)
