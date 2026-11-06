package spacer

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

// contextExportAttempt owns the temporary inductive certificate until the
// source get_model/get_answer result can be transferred to its caller.
type contextExportAttempt struct {
	manager  *ast.ASTManager
	runtime  *memory.Runtime
	property *InductiveProperty
	model    *model.Model
	answer   ast.ASTRef
	nested   memory.ConstructionFailure
}

func (a *contextExportAttempt) closeProperty() error {
	if a.property != nil {
		// Refuse native AST admission before entering the certificate's root loop.
		if n := len(a.property.roots); n != 0 {
			if _, err := a.manager.GetKind(a.property.roots[n-1]); err != nil {
				return err
			}
		}
		if err := a.property.Close(); err != nil {
			return err
		}
		a.property = nil
	}
	return nil
}
func (a *contextExportAttempt) close() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if a.answer != (ast.ASTRef{}) {
		if err := a.manager.DelRef(&a.answer); err != nil {
			return err
		}
	}
	if err := a.closeProperty(); err != nil {
		return err
	}
	if a.model != nil {
		if err := a.model.Close(); err != nil {
			return err
		}
		a.model = nil
	}
	return nil
}
func (a *contextExportAttempt) finish(cause error) error {
	if cause == nil {
		cause = a.closeProperty()
	}
	if cause == nil {
		return nil
	}
	errors.As(cause, &a.nested)
	cleanup := a.close()
	if cleanup == nil {
		return cause
	}
	reported := cause
	if !errors.Is(cause, cleanup) {
		reported = errors.Join(cause, cleanup)
	}
	return &contextExportFailure{attempt: a, original: cause, cleanup: cleanup, reported: reported, text: reported.Error()}
}

type contextExportFailure struct {
	attempt                     *contextExportAttempt
	original, cleanup, reported error
	text                        string
}

func (f *contextExportFailure) Error() string                 { return f.text }
func (f *contextExportFailure) Unwrap() error                 { return f.reported }
func (f *contextExportFailure) OriginalCause() error          { return f.original }
func (f *contextExportFailure) CleanupCause() error           { return f.cleanup }
func (f *contextExportFailure) RuntimeOwner() *memory.Runtime { return f.attempt.runtime }
func (f *contextExportFailure) Cleanup() error                { f.cleanup = f.attempt.close(); return f.cleanup }

var _ memory.ConstructionFailure = (*contextExportFailure)(nil)
