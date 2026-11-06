package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/memory"
)

// Construction owns partial components before invoking another fallible step.
// Nested native cleanup drains before borrowed managers or pinned sorts retire.
type udocTableProductAttempt struct {
	manager *RelationManager
	product *UDocTableProduct
	fullDoc *UDocSieve
	inners  []*TypedUDocRelation
	tables  []FiniteTable
	nested  memory.ConstructionFailure
}

func newUDocTableProductAttempt(manager *RelationManager) *udocTableProductAttempt {
	return &udocTableProductAttempt{manager: manager, product: &UDocTableProduct{manager: manager}}
}

func (a *udocTableProductAttempt) closeTemps() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if a.fullDoc != nil {
		if err := a.fullDoc.Close(); err != nil {
			return err
		}
		a.fullDoc = nil
	}
	for len(a.inners) != 0 {
		last := len(a.inners) - 1
		if err := a.inners[last].Close(); err != nil {
			return err
		}
		a.inners = a.inners[:last]
	}
	for len(a.tables) != 0 {
		last := len(a.tables) - 1
		if err := a.tables[last].Close(); err != nil {
			return err
		}
		a.tables = a.tables[:last]
	}
	return nil
}

func (a *udocTableProductAttempt) close() error {
	if err := a.closeTemps(); err != nil {
		return err
	}
	if a.product != nil {
		if err := a.product.Close(); err != nil {
			return err
		}
		a.product = nil
	}
	return nil
}

type udocTableProductFailure struct {
	owner                       *udocTableProductAttempt
	original, cleanup, reported error
	text                        string
}

func (f *udocTableProductFailure) Error() string        { return f.text }
func (f *udocTableProductFailure) Unwrap() error        { return f.reported }
func (f *udocTableProductFailure) OriginalCause() error { return f.original }
func (f *udocTableProductFailure) CleanupCause() error  { return f.cleanup }
func (f *udocTableProductFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.manager.NumeralService().RuntimeOwner()
}
func (f *udocTableProductFailure) Cleanup() error { f.cleanup = f.owner.close(); return f.cleanup }

func (a *udocTableProductAttempt) finish(cause error) (*UDocTableProduct, error) {
	if cause != nil {
		errors.As(cause, &a.nested)
	}
	cleanup := a.closeTemps()
	if cleanup == nil && cause != nil {
		cleanup = a.close()
	}
	if cleanup != nil {
		original := cause
		if original == nil {
			original = cleanup
		}
		reported := errors.Join(cause, cleanup)
		return nil, &udocTableProductFailure{owner: a, original: original, cleanup: cleanup, reported: reported, text: reported.Error()}
	}
	if cause != nil {
		return nil, cause
	}
	result := a.product
	a.product = nil
	return result, nil
}

// discard finalizes a temporary mutation owner without publishing its delta.
// The mutation itself is already applied; Cleanup only retries destruction.
func (a *udocTableProductAttempt) discard(cause error) error {
	if cause != nil {
		errors.As(cause, &a.nested)
	}
	cleanup := a.close()
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &udocTableProductFailure{owner: a, original: original, cleanup: cleanup, reported: reported, text: reported.Error()}
}
