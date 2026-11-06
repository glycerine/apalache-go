package rel

// Source: dl_sieve_relation.cpp storage, singleton capability selection and
// full(inner_plugin), specialized to the raw DOC component used by product join.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// UDocSieve owns its DOC component and original signature sorts. Columns outside
// the mask denote their full domains; the DOC factory is borrowed.
type UDocSieve struct {
	manager      *ast.ASTManager
	sorts        []ast.ASTHandle
	sortRefs     []ast.ASTRef
	innerColumns []bool
	sigToInner   []int
	innerToSig   []uint32
	inner        *TypedUDocRelation
}

// AdoptUDocSieve transfers inner only on success. The mask may ignore supported
// sorts as well as unsupported ones, matching source mk_from_inner.
func AdoptUDocSieve(sorts []ast.ASTHandle, mask []bool, inner *TypedUDocRelation) (*UDocSieve, error) {
	if inner == nil || inner.open() != nil || len(sorts) != len(mask) {
		return nil, ErrSieve
	}
	r := &UDocSieve{manager: inner.factory.manager, sorts: append([]ast.ASTHandle(nil), sorts...), innerColumns: append([]bool(nil), mask...), sigToInner: make([]int, len(sorts))}
	for i, sort := range sorts {
		kind, err := inner.factory.manager.GetKind(sort)
		if err != nil || kind != ast.ASTSort {
			return nil, errors.Join(err, ast.ErrASTDomain)
		}
		r.sigToInner[i] = -1
		if !mask[i] {
			continue
		}
		index := len(r.innerToSig)
		if index >= len(inner.sorts) || inner.sorts[index] != sort {
			return nil, ErrSieve
		}
		r.sigToInner[i] = index
		r.innerToSig = append(r.innerToSig, uint32(i))
	}
	if len(r.innerToSig) != len(inner.sorts) {
		return nil, ErrSieve
	}
	for _, sort := range sorts {
		ref, err := r.manager.NewRef(sort)
		if ref != (ast.ASTRef{}) {
			r.sortRefs = append(r.sortRefs, ref)
		}
		if err != nil {
			return nil, finishDecodedFact(r.manager, r.sortRefs, err)
		}
	}
	r.inner = inner
	return r, nil
}

func udocSieveSignature(factory *UDocRelationFactory, sorts []ast.ASTHandle) ([]ast.ASTHandle, []bool, error) {
	if factory == nil || factory.open() != nil {
		return nil, nil, ErrSieve
	}
	mask := make([]bool, len(sorts))
	var selected []ast.ASTHandle
	for i, sort := range sorts {
		kind, err := factory.manager.GetKind(sort)
		if err != nil || kind != ast.ASTSort {
			return nil, nil, errors.Join(err, ast.ErrASTDomain)
		}
		mask[i], err = factory.CanHandleSignature([]ast.ASTHandle{sort})
		if err != nil {
			return nil, nil, err
		}
		if mask[i] {
			selected = append(selected, sort)
		}
	}
	return selected, mask, nil
}

func NewEmptyUDocSieve(factory *UDocRelationFactory, sorts []ast.ASTHandle) (*UDocSieve, error) {
	selected, mask, err := udocSieveSignature(factory, sorts)
	if err != nil {
		return nil, err
	}
	inner, err := factory.NewEmpty(selected)
	return finishUDocSieve(sorts, mask, inner, err)
}

// NewFullUDocSieve implements source full(signature, inner_plugin), selecting
// each DOC-capable column; all-selected, all-ignored and nullary masks are valid.
func NewFullUDocSieve(factory *UDocRelationFactory, sorts []ast.ASTHandle) (*UDocSieve, error) {
	selected, mask, err := udocSieveSignature(factory, sorts)
	if err != nil {
		return nil, err
	}
	inner, err := factory.NewFull(selected)
	return finishUDocSieve(sorts, mask, inner, err)
}

func finishUDocSieve(sorts []ast.ASTHandle, mask []bool, inner *TypedUDocRelation, cause error) (*UDocSieve, error) {
	if cause != nil {
		if inner != nil {
			cause = finishCheckedUDocConstruction(cause, inner, ast.ASTRef{})
		}
		return nil, cause
	}
	r, err := AdoptUDocSieve(sorts, mask, inner)
	if err != nil {
		return nil, finishCheckedUDocConstruction(err, inner, ast.ASTRef{})
	}
	return r, nil
}

func (r *UDocSieve) open() error {
	if r == nil || r.inner == nil || r.inner.open() != nil {
		return ErrSieve
	}
	return nil
}

func (r *UDocSieve) Close() error {
	if r == nil {
		return nil
	}
	if r.inner != nil {
		if err := r.inner.Close(); err != nil {
			return err
		}
		r.inner = nil
	}
	for len(r.sortRefs) > 0 {
		last := len(r.sortRefs) - 1
		if r.sortRefs[last] == (ast.ASTRef{}) {
			r.sortRefs = r.sortRefs[:last]
			continue
		}
		err := r.manager.DelRef(&r.sortRefs[last])
		if r.sortRefs[last] == (ast.ASTRef{}) {
			r.sortRefs = r.sortRefs[:last]
		}
		if err != nil {
			return err
		}
	}
	r.manager = nil
	r.sorts, r.innerColumns, r.sigToInner, r.innerToSig = nil, nil, nil, nil
	return nil
}

func (r *UDocSieve) Sorts() []ast.ASTHandle {
	if r.open() != nil {
		return nil
	}
	return append([]ast.ASTHandle(nil), r.sorts...)
}

func (r *UDocSieve) InnerColumns() []bool {
	if r.open() != nil {
		return nil
	}
	return append([]bool(nil), r.innerColumns...)
}

func (r *UDocSieve) Inner() *TypedUDocRelation {
	if r.open() != nil {
		return nil
	}
	return r.inner
}

func (r *UDocSieve) Empty() (bool, error) {
	if err := r.open(); err != nil {
		return false, err
	}
	return r.inner.Empty()
}

func (r *UDocSieve) FastEmpty() (bool, error) {
	if err := r.open(); err != nil {
		return false, err
	}
	return r.inner.FastEmpty()
}

func (r *UDocSieve) fact(values []ast.ASTHandle) ([]ast.ASTHandle, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if len(values) != len(r.sorts) {
		return nil, ast.ErrASTDomain
	}
	selected := make([]ast.ASTHandle, len(r.innerToSig))
	for i, value := range values {
		sort, err := r.inner.factory.manager.GetSort(value)
		if err != nil || sort != r.sorts[i] {
			return nil, errors.Join(err, ast.ErrASTDomain)
		}
		if r.sigToInner[i] >= 0 {
			selected[r.sigToInner[i]] = value
		}
	}
	return selected, nil
}

func (r *UDocSieve) AddFact(values []ast.ASTHandle) (bool, error) {
	selected, err := r.fact(values)
	if err != nil {
		return false, err
	}
	return r.inner.AddFact(selected)
}

func (r *UDocSieve) ContainsFact(values []ast.ASTHandle) (bool, error) {
	selected, err := r.fact(values)
	if err != nil {
		return false, err
	}
	return r.inner.ContainsFact(selected)
}

func (r *UDocSieve) Clone() (*UDocSieve, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	inner, err := r.inner.Clone()
	return finishUDocSieve(r.sorts, r.innerColumns, inner, err)
}
