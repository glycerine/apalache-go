package qe

// Source: src/muz/spacer/spacer_qe_project.cpp peq.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
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
			resultErr = errors.Join(resultErr, result.Close())
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
	if p.decl != (ast.ASTRef{}) {
		result = errors.Join(result, p.manager.DelRef(&p.decl))
	}
	for index := len(p.indices) - 1; index >= 0; index-- {
		result = errors.Join(result, p.manager.DelRef(&p.indices[index]))
	}
	p.indices = nil
	if p.rhs != (ast.ASTRef{}) {
		result = errors.Join(result, p.manager.DelRef(&p.rhs))
	}
	if p.lhs != (ast.ASTRef{}) {
		result = errors.Join(result, p.manager.DelRef(&p.lhs))
	}
	if result == nil {
		p.manager = nil
	}
	return result
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
			resultErr = errors.Join(resultErr, result.Close())
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
	if r.equality != (ast.ASTRef{}) {
		result = errors.Join(result, r.manager.DelRef(&r.equality))
	}
	for index := len(r.constants) - 1; index >= 0; index-- {
		result = errors.Join(result, r.manager.DelRef(&r.constants[index]))
	}
	r.constants = nil
	if result == nil {
		r.manager = nil
	}
	return result
}
