package spacer

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

// literalExpansionListOwner owns speculative expand_literals results until the
// generalized lemma has acquired its final cube.
type literalExpansionListOwner struct {
	manager    *ast.ASTManager
	runtime    *memory.Runtime
	nested     memory.ConstructionFailure
	held       []ast.ASTRef
	expansions []*literalExpansion
	lemma      *Lemma
}

func (o *literalExpansionListOwner) adopt(expansion *literalExpansion) {
	if expansion != nil {
		o.expansions = append(o.expansions, expansion)
	}
}

func (o *literalExpansionListOwner) adoptRefs(refs []ast.ASTRef) {
	o.held = append(o.held, refs...)
}

func (o *literalExpansionListOwner) adoptFailure(cause error) {
	if o.nested == nil && cause != nil {
		_ = errors.As(cause, &o.nested)
	}
}

func (o *literalExpansionListOwner) closeHeld() error {
	for len(o.held) != 0 {
		index := len(o.held) - 1
		if o.held[index] == (ast.ASTRef{}) {
			o.held = o.held[:index]
			continue
		}
		err := o.manager.DelRef(&o.held[index])
		if o.held[index] == (ast.ASTRef{}) {
			o.held = o.held[:index]
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *literalExpansionListOwner) closeExpansions() error {
	for len(o.expansions) != 0 {
		index := len(o.expansions) - 1
		expansion := o.expansions[index]
		err := expansion.Close()
		if len(expansion.refs) == 0 {
			o.expansions[index] = nil
			o.expansions = o.expansions[:index]
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *literalExpansionListOwner) closeTemporary() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	// held was the later source scope and therefore unwinds first.
	if err := o.closeHeld(); err != nil {
		return err
	}
	return o.closeExpansions()
}

func (o *literalExpansionListOwner) close() error {
	if err := o.closeTemporary(); err != nil {
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

func lemmaRootsReleased(lemma *Lemma) bool {
	if lemma.body != (ast.ASTRef{}) {
		return false
	}
	for _, ref := range lemma.bindings {
		if ref != (ast.ASTRef{}) {
			return false
		}
	}
	for _, ref := range lemma.cube {
		if ref != (ast.ASTRef{}) {
			return false
		}
	}
	return true
}

type literalExpansionListCleanupFailure struct {
	owner             *literalExpansionListOwner
	original, cleanup error
	reported          error
	text              string
}

var _ memory.ConstructionFailure = (*literalExpansionListCleanupFailure)(nil)

func (f *literalExpansionListCleanupFailure) Error() string        { return f.text }
func (f *literalExpansionListCleanupFailure) Unwrap() error        { return f.reported }
func (f *literalExpansionListCleanupFailure) OriginalCause() error { return f.original }
func (f *literalExpansionListCleanupFailure) CleanupCause() error  { return f.cleanup }
func (f *literalExpansionListCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.runtime
}
func (f *literalExpansionListCleanupFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

func newLiteralExpansionListCleanupFailure(owner *literalExpansionListOwner, cause, cleanup error) error {
	original := cause
	if original == nil {
		original = cleanup
	}
	reported := cleanup
	if cause != nil && !errors.Is(cause, cleanup) {
		reported = errors.Join(cause, cleanup)
	} else if cause != nil {
		reported = cause
	}
	return &literalExpansionListCleanupFailure{
		owner: owner, original: original, cleanup: cleanup,
		reported: reported, text: reported.Error(),
	}
}

func finishLiteralExpansionList(owner *literalExpansionListOwner, cause error) error {
	owner.adoptFailure(cause)
	cleanup := owner.close()
	if cleanup == nil {
		return cause
	}
	return newLiteralExpansionListCleanupFailure(owner, cause, cleanup)
}

// finishInductiveGeneralization publishes a fresh lemma only after all source
// cube and extra_lits owners have unwound. A cleanup refusal transfers the
// unpublished lemma into the retryable receipt instead of returning it with an
// error that the caller would discard.
func finishInductiveGeneralization(owner *literalExpansionListOwner, original, result *Lemma, cause error) (*Lemma, error) {
	if result != nil && result != original {
		owner.lemma = result
		result = nil
	}
	if cause != nil {
		return result, finishLiteralExpansionList(owner, cause)
	}
	cleanup := owner.closeTemporary()
	if cleanup != nil {
		return result, newLiteralExpansionListCleanupFailure(owner, nil, cleanup)
	}
	if owner.lemma != nil {
		result = owner.lemma
		owner.lemma = nil
	}
	return result, nil
}
