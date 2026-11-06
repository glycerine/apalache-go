package fp

// Concrete retry ownership for the live tr_sparse + interval_relation
// Datalog instantiation. These owners are deliberately specific to this
// backend rather than a generic relation/template cleanup adapter.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
)

type sparseIntervalDatalogConstructionAttempt struct {
	manager    *ast.ASTManager
	nested     memory.ConstructionFailure
	store      *SparseIntervalDatalogStore
	evaluator  *SparseIntervalDatalogEvaluator
	activation *sparseIntervalDatalogActivation
	entries    []*sparseIntervalDatalogStoreEntry
	products   []*rel.SparseIntervalSieveProduct
}

func (a *sparseIntervalDatalogConstructionAttempt) Close() (result error) {
	if a == nil {
		return nil
	}
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if a.activation != nil {
		if err := a.activation.close(); err != nil {
			result = errors.Join(result, err)
		} else {
			a.activation = nil
		}
	}
	if a.evaluator != nil {
		if err := a.evaluator.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			a.evaluator = nil
		}
	}
	if a.store != nil {
		if err := a.store.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			a.store = nil
		}
	}
	for i := len(a.entries) - 1; i >= 0; i-- {
		entry := a.entries[i]
		if entry == nil {
			continue
		}
		if entry.relation != nil {
			if err := entry.relation.Close(); err != nil {
				result = errors.Join(result, err)
				continue
			}
			entry.relation = nil
		}
		if entry.root != (ast.ASTRef{}) {
			if err := a.manager.DelRef(&entry.root); err != nil {
				result = errors.Join(result, err)
				if entry.root != (ast.ASTRef{}) {
					continue
				}
			}
		}
		a.entries[i] = nil
	}
	for i := len(a.products) - 1; i >= 0; i-- {
		if a.products[i] == nil {
			continue
		}
		if err := a.products[i].Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			a.products[i] = nil
		}
	}
	if result == nil {
		a.entries = nil
		a.products = nil
	}
	return result
}

type sparseIntervalDatalogConstructionFailure struct {
	original error
	cleanup  error
	reported error
	text     string
	attempt  *sparseIntervalDatalogConstructionAttempt
}

var _ memory.ConstructionFailure = (*sparseIntervalDatalogConstructionFailure)(nil)

func (f *sparseIntervalDatalogConstructionFailure) Error() string        { return f.text }
func (f *sparseIntervalDatalogConstructionFailure) Unwrap() error        { return f.reported }
func (f *sparseIntervalDatalogConstructionFailure) OriginalCause() error { return f.original }
func (f *sparseIntervalDatalogConstructionFailure) CleanupCause() error  { return f.cleanup }
func (f *sparseIntervalDatalogConstructionFailure) RuntimeOwner() *memory.Runtime {
	if f == nil || f.attempt == nil || f.attempt.manager == nil || f.attempt.manager.NumeralService() == nil {
		return nil
	}
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *sparseIntervalDatalogConstructionFailure) Cleanup() error {
	f.cleanup = f.attempt.Close()
	return f.cleanup
}

func (a *sparseIntervalDatalogConstructionAttempt) dispose(cause error) error {
	if cause != nil {
		errors.As(cause, &a.nested)
	}
	cleanup := a.Close()
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	failure := &sparseIntervalDatalogConstructionFailure{
		original: original,
		cleanup:  cleanup,
		reported: reported,
		text:     reported.Error(),
		attempt:  a,
	}
	return failure
}

func finishSparseIntervalStoreConstruction(cause error, store *SparseIntervalDatalogStore) error {
	if store == nil {
		return cause
	}
	return (&sparseIntervalDatalogConstructionAttempt{manager: store.manager, store: store}).dispose(cause)
}

func finishSparseIntervalEvaluatorConstruction(cause error, evaluator *SparseIntervalDatalogEvaluator) error {
	if evaluator == nil {
		return cause
	}
	return (&sparseIntervalDatalogConstructionAttempt{manager: evaluator.manager, evaluator: evaluator}).dispose(cause)
}

func finishSparseIntervalActivationConstruction(cause error, activation *sparseIntervalDatalogActivation) error {
	if activation == nil {
		return cause
	}
	return (&sparseIntervalDatalogConstructionAttempt{manager: activation.manager, activation: activation}).dispose(cause)
}

func finishSparseIntervalProductConstruction(cause error, manager *ast.ASTManager, products ...*rel.SparseIntervalSieveProduct) error {
	return (&sparseIntervalDatalogConstructionAttempt{manager: manager, products: products}).dispose(cause)
}

func finishSparseIntervalEntriesConstruction(cause error, manager *ast.ASTManager, entries ...*sparseIntervalDatalogStoreEntry) error {
	return (&sparseIntervalDatalogConstructionAttempt{manager: manager, entries: entries}).dispose(cause)
}
