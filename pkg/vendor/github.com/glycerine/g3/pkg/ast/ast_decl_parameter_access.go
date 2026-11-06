package ast

// Source: src/ast/ast.h func_decl_info parameter access;
// src/ast/ast.cpp family_manager::get_family_id.

import usymbol "github.com/glycerine/g3/pkg/util/symbol"

// GetDeclNumParameters reads the source declaration metadata without copying
// any parameter or acquiring references to its AST/numeral payloads.
func (m *ASTManager) GetDeclNumParameters(decl ASTHandle) (uint32, error) {
	if err := m.lock(); err != nil {
		return 0, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(decl, ASTFuncDecl)
	if err != nil {
		return 0, err
	}
	info := m.declAt(r.slot).info
	if info == (ASTFuncDeclInfo{}) {
		return 0, nil
	}
	v, err := m.funcInfo(info)
	if err != nil {
		return 0, err
	}
	return v.count, nil
}

// GetDeclParameterCopy owns only the selected parameter storage and any copied
// numeral; AST payloads borrow the declaration, as in C++ parameter copies.
// Unrelated entries are neither copied nor inspected. The caller releases the
// result with DelParameter while keeping the declaration alive for AST reads.
func (m *ASTManager) GetDeclParameterCopy(decl ASTHandle, index uint32) (ASTParameter, error) {
	if err := m.lock(); err != nil {
		return ASTParameter{}, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(decl, ASTFuncDecl)
	if err != nil {
		return ASTParameter{}, err
	}
	info := m.declAt(r.slot).info
	if info == (ASTFuncDeclInfo{}) {
		return ASTParameter{}, ErrASTDomain
	}
	v, err := m.funcInfo(info)
	if err != nil {
		return ASTParameter{}, err
	}
	if index >= v.count {
		return ASTParameter{}, ErrASTDomain
	}
	parameter, err := m.infoParameter(v, index)
	if err != nil {
		return ASTParameter{}, err
	}
	return m.copyParameter(parameter)
}

// GetDeclParametersCopy returns owning parameter copies. The caller must call
// DelParameter on every returned element.
func (m *ASTManager) GetDeclParametersCopy(decl ASTHandle) (result []ASTParameter, err error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(decl, ASTFuncDecl)
	if e != nil {
		return nil, e
	}
	d := m.declAt(r.slot)
	if d.info.slot == 0 {
		return nil, nil
	}
	info := m.infoAt(d.info.slot)
	for i := uint32(0); i < info.count; i++ {
		p, e := m.infoParameter(info, i)
		if e != nil {
			for j := range result {
				m.dropParameter(result[j])
			}
			return nil, e
		}
		copy, e := m.copyParameter(p)
		if e != nil {
			for j := range result {
				m.dropParameter(result[j])
			}
			return nil, e
		}
		result = append(result, copy)
	}
	return result, nil
}

func (m *ASTManager) FamilyIDForSymbol(name usymbol.Symbol) (FamilyID, error) {
	if e := m.lock(); e != nil {
		return NullFamilyID, e
	}
	defer m.mu.Unlock()
	return m.families.GetID(name)
}

// FamilyName returns the borrowed symbol from ast_manager::get_family_name.
func (m *ASTManager) FamilyName(family FamilyID) (usymbol.Symbol, error) {
	if err := m.lock(); err != nil {
		return usymbol.Symbol{}, err
	}
	defer m.mu.Unlock()
	return m.families.GetName(family)
}
