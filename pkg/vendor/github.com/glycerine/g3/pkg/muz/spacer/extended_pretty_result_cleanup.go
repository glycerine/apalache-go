package spacer

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

// extendedPrettyResultOwner holds the rewritten expression while the source
// trace consumer formats and writes it.
type extendedPrettyResultOwner struct {
	manager *ast.ASTManager
	value   ast.ASTRef
}

func (o *extendedPrettyResultOwner) close() error {
	if o.value == (ast.ASTRef{}) {
		return nil
	}
	return o.manager.DelRef(&o.value)
}

type extendedPrettyResultCleanupFailure struct {
	owner             *extendedPrettyResultOwner
	original, cleanup error
	reported          error
	text              string
}

var _ memory.ConstructionFailure = (*extendedPrettyResultCleanupFailure)(nil)

func (f *extendedPrettyResultCleanupFailure) Error() string        { return f.text }
func (f *extendedPrettyResultCleanupFailure) Unwrap() error        { return f.reported }
func (f *extendedPrettyResultCleanupFailure) OriginalCause() error { return f.original }
func (f *extendedPrettyResultCleanupFailure) CleanupCause() error  { return f.cleanup }
func (f *extendedPrettyResultCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *extendedPrettyResultCleanupFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

func finishExtendedPrettyResult(owner *extendedPrettyResultOwner, cause error) error {
	cleanup := owner.close()
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &extendedPrettyResultCleanupFailure{
		owner: owner, original: original, cleanup: cleanup,
		reported: reported, text: reported.Error(),
	}
}
