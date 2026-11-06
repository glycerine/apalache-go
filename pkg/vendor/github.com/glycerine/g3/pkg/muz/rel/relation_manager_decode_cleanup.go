package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type decodedFactCleanup struct {
	manager                     *ast.ASTManager
	runtime                     *memory.Runtime
	references                  []ast.ASTRef
	nested                      memory.ConstructionFailure
	original, cleanup, reported error
	text                        string
}

func (f *decodedFactCleanup) Error() string                 { return f.text }
func (f *decodedFactCleanup) Unwrap() error                 { return f.reported }
func (f *decodedFactCleanup) OriginalCause() error          { return f.original }
func (f *decodedFactCleanup) CleanupCause() error           { return f.cleanup }
func (f *decodedFactCleanup) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *decodedFactCleanup) Cleanup() error {
	f.cleanup = f.close()
	return f.cleanup
}
func (f *decodedFactCleanup) close() error {
	if f.nested != nil {
		if err := f.nested.Cleanup(); err != nil {
			return err
		}
		f.nested = nil
	}
	for len(f.references) > 0 {
		i := len(f.references) - 1
		if f.references[i] != (ast.ASTRef{}) {
			if err := f.manager.DelRef(&f.references[i]); err != nil {
				return err
			}
		}
		f.references = f.references[:i]
	}
	return nil
}
func finishDecodedFact(manager *ast.ASTManager, references []ast.ASTRef, cause error) error {
	f := &decodedFactCleanup{manager: manager, runtime: manager.NumeralService().RuntimeOwner(), references: references, original: cause}
	errors.As(cause, &f.nested)
	if f.cleanup = f.close(); f.cleanup != nil {
		f.reported = errors.Join(cause, f.cleanup)
		f.text = f.reported.Error()
		return f
	}
	return cause
}
