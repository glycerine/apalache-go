package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/memory"
)

// formulaCheckedTableConstructionFailure preserves an unpublished checked
// relation when native AST release refuses during constructor cleanup.
type formulaCheckedTableConstructionFailure struct {
	owner             *FormulaCheckedTable
	nested            memory.ConstructionFailure
	runtime           *memory.Runtime
	original, cleanup error
	reported          error
	text              string
}

func (f *formulaCheckedTableConstructionFailure) Error() string { return f.text }
func (f *formulaCheckedTableConstructionFailure) Unwrap() error { return f.reported }
func (f *formulaCheckedTableConstructionFailure) RuntimeOwner() *memory.Runtime {
	return f.runtime
}
func (f *formulaCheckedTableConstructionFailure) OriginalCause() error { return f.original }
func (f *formulaCheckedTableConstructionFailure) CleanupCause() error  { return f.cleanup }
func (f *formulaCheckedTableConstructionFailure) Cleanup() error {
	if f.nested != nil {
		f.cleanup = f.nested.Cleanup()
		if f.cleanup != nil {
			return f.cleanup
		}
		f.nested = nil
	}
	if f.owner == nil {
		f.cleanup = nil
		return nil
	}
	f.cleanup = f.owner.Close()
	if f.cleanup == nil {
		f.owner = nil
	}
	return f.cleanup
}

func finishFormulaCheckedTableConstruction(cause error, owner *FormulaCheckedTable) error {
	if owner == nil {
		return cause
	}
	var runtime *memory.Runtime
	if owner.manager != nil {
		runtime = owner.manager.NumeralService().RuntimeOwner()
	}
	var nested memory.ConstructionFailure
	if cause != nil {
		errors.As(cause, &nested)
		if nested != nil && nested.RuntimeOwner() != nil {
			runtime = nested.RuntimeOwner()
		}
	}
	var cleanup error
	if nested != nil {
		cleanup = nested.Cleanup()
		if cleanup == nil {
			nested = nil
		}
	}
	if cleanup == nil {
		cleanup = owner.Close()
	}
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &formulaCheckedTableConstructionFailure{
		owner: owner, nested: nested, runtime: runtime, original: original, cleanup: cleanup,
		reported: reported, text: reported.Error(),
	}
}

var _ memory.ConstructionFailure = (*formulaCheckedTableConstructionFailure)(nil)
