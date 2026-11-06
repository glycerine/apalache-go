package spacer

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type equalityExpansionOwner struct {
	manager     *ast.ASTManager
	equivalence *ast.ExprEquivClass
	refs        []ast.ASTRef
	nested      memory.ConstructionFailure
}

func (o *equalityExpansionOwner) closeRefs() error {
	for len(o.refs) != 0 {
		index := len(o.refs) - 1
		if o.refs[index] == (ast.ASTRef{}) {
			o.refs = o.refs[:index]
			continue
		}
		err := o.manager.DelRef(&o.refs[index])
		if o.refs[index] == (ast.ASTRef{}) {
			o.refs = o.refs[:index]
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *equalityExpansionOwner) close() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	if err := o.closeRefs(); err != nil {
		return err
	}
	if o.equivalence != nil {
		if err := o.equivalence.Close(); err != nil {
			return err
		}
		o.equivalence = nil
	}
	return nil
}

type equalityExpansionCleanupFailure struct {
	owner             *equalityExpansionOwner
	original, cleanup error
	reported          error
	text              string
}

var _ memory.ConstructionFailure = (*equalityExpansionCleanupFailure)(nil)

func (f *equalityExpansionCleanupFailure) Error() string        { return f.text }
func (f *equalityExpansionCleanupFailure) Unwrap() error        { return f.reported }
func (f *equalityExpansionCleanupFailure) OriginalCause() error { return f.original }
func (f *equalityExpansionCleanupFailure) CleanupCause() error  { return f.cleanup }
func (f *equalityExpansionCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *equalityExpansionCleanupFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

func finishEqualityExpansion(owner *equalityExpansionOwner, cause error) error {
	if cause != nil {
		errors.As(cause, &owner.nested)
	}
	if cause == nil {
		if owner.equivalence == nil {
			owner.refs = nil
			return nil
		}
		cleanup := owner.equivalence.Close()
		if cleanup == nil {
			owner.equivalence = nil
			owner.refs = nil
			return nil
		}
		return &equalityExpansionCleanupFailure{
			owner: owner, original: cleanup, cleanup: cleanup,
			reported: cleanup, text: cleanup.Error(),
		}
	}
	cleanup := owner.close()
	if cleanup == nil {
		return cause
	}
	reported := errors.Join(cause, cleanup)
	return &equalityExpansionCleanupFailure{
		owner: owner, original: cause, cleanup: cleanup,
		reported: reported, text: reported.Error(),
	}
}

// equalityGeneralizationOwner keeps the expanded cube and a newly built lemma
// private until destruction of the temporary cube has completed. A failed
// cleanup therefore cannot publish a lemma whose construction inputs remain
// owned by an unreachable defer frame.
type equalityGeneralizationOwner struct {
	manager *ast.ASTManager
	refs    []ast.ASTRef
	lemma   *Lemma
	nested  memory.ConstructionFailure
}

func (o *equalityGeneralizationOwner) closeRefs() error {
	for len(o.refs) != 0 {
		index := len(o.refs) - 1
		if o.refs[index] == (ast.ASTRef{}) {
			o.refs = o.refs[:index]
			continue
		}
		err := o.manager.DelRef(&o.refs[index])
		if o.refs[index] == (ast.ASTRef{}) {
			o.refs = o.refs[:index]
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *equalityGeneralizationOwner) close() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	if err := o.closeRefs(); err != nil {
		return err
	}
	if o.lemma != nil {
		lemma := o.lemma
		if err := lemma.Close(); err != nil {
			if lemmaRootsReleased(lemma) {
				lemma.bindings = nil
				lemma.cube = nil
				lemma.manager = nil
				lemma.context = nil
				lemma.globals = nil
				lemma.closed = true
				o.lemma = nil
			}
			return err
		}
		o.lemma = nil
	}
	return nil
}

type equalityGeneralizationCleanupFailure struct {
	owner             *equalityGeneralizationOwner
	original, cleanup error
	reported          error
	text              string
}

var _ memory.ConstructionFailure = (*equalityGeneralizationCleanupFailure)(nil)

func (f *equalityGeneralizationCleanupFailure) Error() string        { return f.text }
func (f *equalityGeneralizationCleanupFailure) Unwrap() error        { return f.reported }
func (f *equalityGeneralizationCleanupFailure) OriginalCause() error { return f.original }
func (f *equalityGeneralizationCleanupFailure) CleanupCause() error  { return f.cleanup }
func (f *equalityGeneralizationCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *equalityGeneralizationCleanupFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

func finishEqualityGeneralization(owner *equalityGeneralizationOwner, cause error, published *Lemma) error {
	if cause != nil {
		errors.As(cause, &owner.nested)
	}
	if cause == nil && published != nil && owner.lemma == published {
		owner.lemma = nil
		cleanup := owner.close()
		if cleanup == nil {
			return nil
		}
		owner.lemma = published
		return &equalityGeneralizationCleanupFailure{
			owner: owner, original: cleanup, cleanup: cleanup,
			reported: cleanup, text: cleanup.Error(),
		}
	}
	cleanup := owner.close()
	if cleanup == nil {
		return cause
	}
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := cause
	if reported == nil {
		reported = cleanup
	} else if !errors.Is(cause, cleanup) {
		reported = errors.Join(cause, cleanup)
	}
	return &equalityGeneralizationCleanupFailure{
		owner: owner, original: original, cleanup: cleanup,
		reported: reported, text: reported.Error(),
	}
}
