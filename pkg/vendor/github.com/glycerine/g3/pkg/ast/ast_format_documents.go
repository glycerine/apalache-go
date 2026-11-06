package ast

// Source: ast_smt2_pp.cpp format_ref/format_ref_vector lexical ownership.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/memory"
)

// astFormatDocuments owns only documents produced by one formatter. Inputs
// remain borrowed. Direct node references cannot introduce a reference-page
// allocation after a producer has returned a fresh zero-reference document.
type astFormatDocuments struct {
	manager *ASTManager
	roots   []astNodeRef
	result  astNodeRef
	// Flat/Group borrow the entire input DAG, which may itself have RC0.
	// Keep it alive while temporary aliases are released, then restore its
	// original count without collecting the caller's raw document.
	borrowedRoot astNodeRef
	nested       memory.ConstructionFailure
}

func newASTFormatDocuments(manager *ASTManager) (*astFormatDocuments, error) {
	formats, err := manager.GetFormatManager()
	if err != nil {
		return nil, err
	}
	return &astFormatDocuments{manager: formats}, nil
}

func (d *astFormatDocuments) own(value ASTHandle, cause error) (ASTHandle, error) {
	if value.IsNull() {
		return value, cause
	}
	if err := d.manager.lock(); err != nil {
		return value, errors.Join(cause, err)
	}
	defer d.manager.mu.Unlock()
	node, err := d.manager.internal(value)
	if err == nil {
		_, err = d.ownNode(node, nil)
	}
	return value, errors.Join(cause, err)
}

// ownNode is the same concrete ownership operation while a primitive format
// producer already holds the format manager lock.
func (d *astFormatDocuments) ownNode(node astNodeRef, cause error) (astNodeRef, error) {
	if node.slot == 0 {
		return node, cause
	}
	err := d.manager.incNode(node)
	if err == nil {
		d.roots = append(d.roots, node)
	}
	return node, errors.Join(cause, err)
}

// borrowRoot is called once with the format manager already locked.
func (d *astFormatDocuments) borrowRoot(node astNodeRef) error {
	if d.borrowedRoot.slot != 0 {
		return ErrASTDomain
	}
	if err := d.manager.incNode(node); err != nil {
		return err
	}
	d.borrowedRoot = node
	return nil
}

func (d *astFormatDocuments) releaseBorrowedRoot() error {
	if d.borrowedRoot.slot == 0 {
		return nil
	}
	if err := d.manager.lock(); err != nil {
		return err
	}
	defer d.manager.mu.Unlock()
	d.manager.nodeAt(d.borrowedRoot.slot).rc--
	d.borrowedRoot = astNodeRef{}
	return nil
}

func (d *astFormatDocuments) closeRoots() error {
	if d.nested != nil {
		if err := d.nested.Cleanup(); err != nil {
			return err
		}
		d.nested = nil
	}
	if err := d.manager.lock(); err != nil {
		return err
	}
	defer d.manager.mu.Unlock()
	for len(d.roots) != 0 {
		last := len(d.roots) - 1
		consumed, err := d.manager.decNode(d.roots[last])
		if consumed {
			d.roots = d.roots[:last]
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *astFormatDocuments) close() error {
	if err := d.closeRoots(); err != nil {
		return err
	}
	if d.result.slot == 0 {
		return d.releaseBorrowedRoot()
	}
	if err := d.manager.lock(); err != nil {
		return err
	}
	consumed, err := d.manager.decNode(d.result)
	if consumed {
		d.result = astNodeRef{}
	}
	d.manager.mu.Unlock()
	if err != nil {
		return err
	}
	return d.releaseBorrowedRoot()
}

// finish preserves the existing raw-result API: a successful output is left
// at its original reference count after its temporary owners have gone away.
// Dropping this final lexical increment must not destroy an RC0 result.
func (d *astFormatDocuments) finish(value *ASTHandle, cause *error) {
	if !value.IsNull() {
		_, err := d.own(*value, nil)
		*cause = errors.Join(*cause, err)
		if err == nil {
			last := len(d.roots) - 1
			d.result = d.roots[last]
			d.roots = d.roots[:last]
		}
	}
	if *cause != nil {
		errors.As(*cause, &d.nested)
	}
	cleanup := d.closeRoots()
	if *cause == nil && cleanup == nil && d.result.slot != 0 {
		if err := d.manager.lock(); err != nil {
			cleanup = err
		} else {
			d.manager.nodeAt(d.result.slot).rc--
			d.result = astNodeRef{}
			if d.borrowedRoot.slot != 0 {
				d.manager.nodeAt(d.borrowedRoot.slot).rc--
				d.borrowedRoot = astNodeRef{}
			}
			d.manager.mu.Unlock()
			return
		}
	}
	if cleanup == nil {
		cleanup = d.close()
	}
	*value = ASTHandle{}
	if cleanup == nil {
		return
	}
	original := *cause
	if original == nil {
		original = cleanup
	}
	reported := errors.Join(*cause, cleanup)
	*cause = &astFormatDocumentsFailure{original: original, cleanup: cleanup, reported: reported, text: reported.Error(), owner: d}
}

type astFormatDocumentsFailure struct {
	original, cleanup, reported error
	text                        string
	owner                       *astFormatDocuments
}

var _ memory.ConstructionFailure = (*astFormatDocumentsFailure)(nil)

func (f *astFormatDocumentsFailure) Error() string        { return f.text }
func (f *astFormatDocumentsFailure) Unwrap() error        { return f.reported }
func (f *astFormatDocumentsFailure) OriginalCause() error { return f.original }
func (f *astFormatDocumentsFailure) CleanupCause() error  { return f.cleanup }
func (f *astFormatDocumentsFailure) RuntimeOwner() *memory.Runtime {
	if numbers := f.owner.manager.NumeralService(); numbers != nil {
		return numbers.RuntimeOwner()
	}
	return nil
}
func (f *astFormatDocumentsFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}
