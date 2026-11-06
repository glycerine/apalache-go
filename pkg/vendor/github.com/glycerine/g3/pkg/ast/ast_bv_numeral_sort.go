package ast

// Source: src/ast/bv_decl_plugin.cpp:774-776,858-864;
// src/ast/bv_decl_plugin.h:390-394.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *ASTManager) bitVectorSortRecognized(s astNodeRef) (bool, error) {
	if m.bitVectorPlugin == nil {
		return false, nil
	}
	info, e := m.sortInfo(m.sortAt(s.slot).info)
	if e != nil {
		return false, e
	}
	return info.family == m.bitVectorPlugin.family && info.kind == 0 && info.count == 1, nil
}

func (m *ASTManager) IsBitVectorSort(s ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(s, ASTSort)
	if e != nil {
		return false, e
	}
	return m.bitVectorSortRecognized(r)
}

func (m *ASTManager) BitVectorNumeralForSort(value *unumeral.OwnedRational, s ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.requireKind(s, ASTSort)
	if e != nil {
		return ASTHandle{}, e
	}
	yes, e := m.bitVectorSortRecognized(r)
	if e != nil || !yes {
		return ASTHandle{}, e
	}
	width, e := m.bitVectorSortSize(r, nil)
	if e != nil {
		return ASTHandle{}, e
	}
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	n, e := m.mkBitVectorNumeral(value, width)
	return m.external(n), e
}
