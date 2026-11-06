package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type storedFactFormulaAttempt struct {
	manager          *ast.ASTManager
	current, outputs []ast.ASTRef
	nested           memory.ConstructionFailure
}

func (a *storedFactFormulaAttempt) clearCurrent() error {
	for len(a.current) > 0 {
		i := len(a.current) - 1
		if a.current[i] != (ast.ASTRef{}) {
			if err := a.manager.DelRef(&a.current[i]); err != nil {
				return err
			}
		}
		a.current = a.current[:i]
	}
	return nil
}
func (a *storedFactFormulaAttempt) close() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if err := a.clearCurrent(); err != nil {
		return err
	}
	for len(a.outputs) > 0 {
		i := len(a.outputs) - 1
		if a.outputs[i] != (ast.ASTRef{}) {
			if err := a.manager.DelRef(&a.outputs[i]); err != nil {
				return err
			}
		}
		a.outputs = a.outputs[:i]
	}
	return nil
}

type storedFactFormulaFailure struct {
	attempt                     *storedFactFormulaAttempt
	runtime                     *memory.Runtime
	original, cleanup, reported error
	text                        string
}

func (f *storedFactFormulaFailure) Error() string                 { return f.text }
func (f *storedFactFormulaFailure) Unwrap() error                 { return f.reported }
func (f *storedFactFormulaFailure) OriginalCause() error          { return f.original }
func (f *storedFactFormulaFailure) CleanupCause() error           { return f.cleanup }
func (f *storedFactFormulaFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *storedFactFormulaFailure) Cleanup() error                { f.cleanup = f.attempt.close(); return f.cleanup }
func (a *storedFactFormulaAttempt) fail(cause error) error {
	errors.As(cause, &a.nested)
	if cleanup := a.close(); cleanup != nil {
		reported := errors.Join(cause, cleanup)
		return &storedFactFormulaFailure{attempt: a, runtime: a.manager.NumeralService().RuntimeOwner(), original: cause, cleanup: cleanup, reported: reported, text: reported.Error()}
	}
	return cause
}
