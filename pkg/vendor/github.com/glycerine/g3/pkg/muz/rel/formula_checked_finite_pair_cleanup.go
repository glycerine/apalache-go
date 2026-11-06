package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/memory"
)

// formulaCheckedFinitePairCleanupAttempt keeps every unpublished nested
// checker owner reachable while native AST release advances. A nested
// construction receipt is retired first because it can still own resources
// referenced by the local composite.
type formulaCheckedFinitePairCleanupAttempt struct {
	nested memory.ConstructionFailure
	owner  *FormulaCheckedFinitePair
}

func (a *formulaCheckedFinitePairCleanupAttempt) close() error {
	if a == nil {
		return nil
	}
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if a.owner != nil {
		if err := a.owner.Close(); err != nil {
			return err
		}
		a.owner = nil
	}
	return nil
}

func (a *formulaCheckedFinitePairCleanupAttempt) runtimeOwner() *memory.Runtime {
	if a == nil {
		return nil
	}
	if a.nested != nil && a.nested.RuntimeOwner() != nil {
		return a.nested.RuntimeOwner()
	}
	if a.owner == nil {
		return nil
	}
	if a.owner.formula != nil && a.owner.formula.manager != nil {
		return a.owner.formula.manager.NumeralService().RuntimeOwner()
	}
	for _, formula := range a.owner.retired {
		if formula != nil && formula.manager != nil {
			return formula.manager.NumeralService().RuntimeOwner()
		}
	}
	return nil
}

type formulaCheckedFinitePairConstructionFailure struct {
	attempt           *formulaCheckedFinitePairCleanupAttempt
	original, cleanup error
	reported          error
	text              string
}

func (f *formulaCheckedFinitePairConstructionFailure) Error() string { return f.text }
func (f *formulaCheckedFinitePairConstructionFailure) Unwrap() error { return f.reported }
func (f *formulaCheckedFinitePairConstructionFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.runtimeOwner()
}
func (f *formulaCheckedFinitePairConstructionFailure) OriginalCause() error { return f.original }
func (f *formulaCheckedFinitePairConstructionFailure) CleanupCause() error  { return f.cleanup }
func (f *formulaCheckedFinitePairConstructionFailure) Cleanup() error {
	f.cleanup = f.attempt.close()
	return f.cleanup
}

func finishFormulaCheckedFinitePairConstruction(cause error, owner *FormulaCheckedFinitePair) error {
	attempt := &formulaCheckedFinitePairCleanupAttempt{owner: owner}
	if cause != nil {
		errors.As(cause, &attempt.nested)
	}
	cleanup := attempt.close()
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &formulaCheckedFinitePairConstructionFailure{
		attempt: attempt, original: original, cleanup: cleanup,
		reported: reported, text: reported.Error(),
	}
}

var _ memory.ConstructionFailure = (*formulaCheckedFinitePairConstructionFailure)(nil)
