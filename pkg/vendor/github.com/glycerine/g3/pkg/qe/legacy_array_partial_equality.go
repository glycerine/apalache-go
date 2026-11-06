package qe

// Source: src/muz/spacer/spacer_qe_project.cpp peq.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type legacyPartialEquality struct {
	manager *ast.ASTManager
	family  ast.FamilyID
	lhs     ast.ASTRef
	rhs     ast.ASTRef
	indices []ast.ASTRef
	decl    ast.ASTRef
}

type legacyPartialEqualityExpansion struct {
	manager   *ast.ASTManager
	equality  ast.ASTRef
	constants []ast.ASTRef
}

type legacyPartialEqualityExpansionFailure struct {
	owner          *legacyPartialEqualityExpansion
	nested         memory.ConstructionFailure
	runtime        *memory.Runtime
	cause, cleanup error
}

var _ memory.ConstructionFailure = (*legacyPartialEqualityExpansionFailure)(nil)

func (f *legacyPartialEqualityExpansionFailure) Error() string {
	return errors.Join(f.cause, f.cleanup).Error()
}
func (f *legacyPartialEqualityExpansionFailure) Unwrap() error {
	return errors.Join(f.cause, f.cleanup)
}
func (f *legacyPartialEqualityExpansionFailure) OriginalCause() error          { return f.cause }
func (f *legacyPartialEqualityExpansionFailure) CleanupCause() error           { return f.cleanup }
func (f *legacyPartialEqualityExpansionFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *legacyPartialEqualityExpansionFailure) Cleanup() error {
	if f.nested != nil {
		f.cleanup = f.nested.Cleanup()
		if f.cleanup != nil {
			return f.cleanup
		}
		f.nested = nil
	}
	if f.owner != nil && f.owner.manager != nil {
		f.cleanup = f.owner.Close()
		if f.cleanup != nil {
			return f.cleanup
		}
	}
	f.owner = nil
	f.cleanup = nil
	return nil
}

func closeFailedLegacyPartialExpansion(owner *legacyPartialEqualityExpansion, cause error) error {
	cleanup := owner.Close()
	if cleanup == nil {
		return cause
	}
	origin := cause
	if origin == nil {
		origin = cleanup
	}
	var nested memory.ConstructionFailure
	_ = errors.As(cause, &nested)
	return &legacyPartialEqualityExpansionFailure{
		owner: owner, nested: nested, runtime: owner.manager.NumeralService().RuntimeOwner(),
		cause: origin, cleanup: cleanup,
	}
}

type legacyPartialEqualityConstructionFailure struct {
	owner          *legacyPartialEquality
	nested         memory.ConstructionFailure
	runtime        *memory.Runtime
	cause, cleanup error
	reported       error
}

var _ memory.ConstructionFailure = (*legacyPartialEqualityConstructionFailure)(nil)

func (f *legacyPartialEqualityConstructionFailure) Error() string                 { return f.reported.Error() }
func (f *legacyPartialEqualityConstructionFailure) Unwrap() error                 { return f.reported }
func (f *legacyPartialEqualityConstructionFailure) OriginalCause() error          { return f.cause }
func (f *legacyPartialEqualityConstructionFailure) CleanupCause() error           { return f.cleanup }
func (f *legacyPartialEqualityConstructionFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *legacyPartialEqualityConstructionFailure) Cleanup() error {
	if f.nested != nil {
		f.cleanup = f.nested.Cleanup()
		if f.cleanup != nil {
			return f.cleanup
		}
		f.nested = nil
	}
	if f.owner == nil || f.owner.manager == nil {
		f.owner = nil
		f.cleanup = nil
		return nil
	}
	f.cleanup = f.owner.Close()
	if f.cleanup == nil {
		f.owner = nil
	}
	return f.cleanup
}

func closeFailedLegacyPartialEquality(owner *legacyPartialEquality, cause error) error {
	cleanup := owner.Close()
	if cleanup == nil {
		return cause
	}
	origin := cause
	if origin == nil {
		origin = cleanup
	}
	reported := cleanup
	if cause != nil {
		reported = errors.Join(cause, cleanup)
	}
	var nested memory.ConstructionFailure
	_ = errors.As(cause, &nested)
	return &legacyPartialEqualityConstructionFailure{
		owner: owner, nested: nested, runtime: owner.manager.NumeralService().RuntimeOwner(),
		cause: origin, cleanup: cleanup, reported: reported,
	}
}

func newLegacyPartialEquality(manager *ast.ASTManager, family ast.FamilyID, lhs, rhs ast.ASTHandle, indices []ast.ASTHandle) (_ *legacyPartialEquality, resultErr error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	leftSort, err := manager.GetSort(lhs)
	if err != nil {
		return nil, err
	}
	rightSort, err := manager.GetSort(rhs)
	if err != nil || leftSort != rightSort {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return nil, err
	}
	result := &legacyPartialEquality{manager: manager, family: family}
	defer func() {
		if resultErr != nil {
			resultErr = closeFailedLegacyPartialEquality(result, resultErr)
		}
	}()
	result.lhs, err = manager.NewRef(lhs)
	if err != nil {
		return nil, err
	}
	result.rhs, err = manager.NewRef(rhs)
	if err != nil {
		return nil, err
	}
	domain := []ast.ASTHandle{leftSort, rightSort}
	for _, index := range indices {
		indexSort, err := manager.GetSort(index)
		if err != nil {
			return nil, err
		}
		domain = append(domain, indexSort)
		indexRef, err := manager.NewRef(index)
		if err != nil {
			return nil, err
		}
		result.indices = append(result.indices, indexRef)
	}
	name, err := manager.Symbols.Intern("partial_eq")
	if err != nil {
		return nil, err
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return nil, err
	}
	decl, err := manager.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		return nil, err
	}
	result.decl, err = manager.NewRef(decl)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (p *legacyPartialEquality) Close() (result error) {
	if p == nil || p.manager == nil {
		return ast.ErrASTHandle
	}
	if _, err := p.manager.ProofMode(); err != nil {
		return err
	}
	if p.decl != (ast.ASTRef{}) {
		if err := p.manager.DelRef(&p.decl); err != nil {
			return err
		}
	}
	for len(p.indices) != 0 {
		index := len(p.indices) - 1
		if p.indices[index] == (ast.ASTRef{}) {
			p.indices = p.indices[:index]
			continue
		}
		if err := p.manager.DelRef(&p.indices[index]); err != nil {
			return err
		}
		p.indices = p.indices[:index]
	}
	if p.rhs != (ast.ASTRef{}) {
		if err := p.manager.DelRef(&p.rhs); err != nil {
			return err
		}
	}
	if p.lhs != (ast.ASTRef{}) {
		if err := p.manager.DelRef(&p.lhs); err != nil {
			return err
		}
	}
	p.manager = nil
	return nil
}

func (p *legacyPartialEquality) Application() (ast.ASTRef, error) {
	if p == nil || p.manager == nil {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	lhs, err := p.manager.RefGet(p.lhs)
	if err != nil {
		return ast.ASTRef{}, err
	}
	rhs, err := p.manager.RefGet(p.rhs)
	if err != nil {
		return ast.ASTRef{}, err
	}
	args := []ast.ASTHandle{lhs, rhs}
	for _, index := range p.indices {
		handle, err := p.manager.RefGet(index)
		if err != nil {
			return ast.ASTRef{}, err
		}
		args = append(args, handle)
	}
	decl, err := p.manager.RefGet(p.decl)
	if err != nil {
		return ast.ASTRef{}, err
	}
	application, err := p.manager.MkApp(decl, args)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.manager.NewRef(application)
}

func (p *legacyPartialEquality) Expand(storesOnRight bool) (_ *legacyPartialEqualityExpansion, resultErr error) {
	if p == nil || p.manager == nil {
		return nil, ast.ErrASTHandle
	}
	result := &legacyPartialEqualityExpansion{manager: p.manager}
	defer func() {
		if resultErr != nil {
			resultErr = closeFailedLegacyPartialExpansion(result, resultErr)
		}
	}()
	lhs, err := p.manager.RefGet(p.lhs)
	if err != nil {
		return nil, err
	}
	rhs, err := p.manager.RefGet(p.rhs)
	if err != nil {
		return nil, err
	}
	if !storesOnRight {
		lhs, rhs = rhs, lhs
	}
	arraySort, err := p.manager.GetSort(lhs)
	if err != nil {
		return nil, err
	}
	rangeSort, err := p.manager.ArrayRange(arraySort)
	if err != nil {
		return nil, err
	}
	for _, indexRef := range p.indices {
		index, err := p.manager.RefGet(indexRef)
		if err != nil {
			return nil, err
		}
		constant, err := p.manager.MkFreshConst("diff", rangeSort)
		if err != nil {
			return nil, err
		}
		constantRef, err := p.manager.NewRef(constant)
		if err != nil {
			return nil, err
		}
		result.constants = append(result.constants, constantRef)
		rhs, err = p.manager.MkFamilyApp(p.family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{rhs, index, constant})
		if err != nil {
			return nil, err
		}
	}
	equality, err := p.manager.MkEq(lhs, rhs)
	if err != nil {
		return nil, err
	}
	result.equality, err = p.manager.NewRef(equality)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *legacyPartialEqualityExpansion) Close() (result error) {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	if _, err := r.manager.ProofMode(); err != nil {
		return err
	}
	if r.equality != (ast.ASTRef{}) {
		if err := r.manager.DelRef(&r.equality); err != nil {
			return err
		}
	}
	for len(r.constants) != 0 {
		index := len(r.constants) - 1
		if err := r.manager.DelRef(&r.constants[index]); err != nil {
			return err
		}
		r.constants = r.constants[:index]
	}
	r.manager = nil
	return nil
}
