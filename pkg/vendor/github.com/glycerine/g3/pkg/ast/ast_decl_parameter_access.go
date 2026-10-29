package ast

// Source: src/ast/ast.h func_decl_info parameter access;
// src/ast/ast.cpp family_manager::get_family_id.

import usymbol "github.com/glycerine/g3/pkg/util/symbol"

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
