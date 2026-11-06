package ast

// Source: src/ast/seq_decl_plugin.cpp:860-923;
// src/ast/seq_decl_plugin.h:234-244.

import "math"

func (m *ASTManager) sequenceValue(fid FamilyID, value astNodeRef) (bool, error) {
	for {
		if m.nodeAt(value.slot).kind != ASTApp {
			return false, nil
		}
		app := m.appAt(value.slot)
		family, kind, err := m.declIdentity(app.decl)
		if err != nil || family != fid {
			return false, err
		}
		switch kind {
		case ASTSequenceOpEmpty, ASTSequenceOpStringConst:
			return true, nil
		case ASTSequenceOpUnit:
			if app.count != 1 {
				return false, ErrASTDomain
			}
			arg, err := m.readInternal(app.args, 0)
			if err != nil {
				return false, err
			}
			return m.isValue(arg)
		case ASTSequenceOpConcat:
			if app.count != 2 {
				return false, ErrASTDomain
			}
			left, err := m.readInternal(app.args, 0)
			if err != nil {
				return false, err
			}
			leftValue, err := m.sequenceValue(fid, left)
			if err != nil || !leftValue {
				return leftValue, err
			}
			value, err = m.readInternal(app.args, 1)
			if err != nil {
				return false, err
			}
		default:
			return false, nil
		}
	}
}

func (m *ASTManager) sequenceDistinct(fid FamilyID, a, b astNodeRef) (bool, error) {
	if a == b {
		return false, nil
	}
	if m.nodeAt(a.slot).kind != ASTApp || m.nodeAt(b.slot).kind != ASTApp {
		return false, nil
	}
	af, ak, err := m.declIdentity(m.appAt(a.slot).decl)
	if err != nil {
		return false, err
	}
	bf, bk, err := m.declIdentity(m.appAt(b.slot).decl)
	if err != nil || af != fid || bf != fid {
		return false, err
	}
	if ak == ASTSequenceOpStringConst && bk == ASTSequenceOpStringConst {
		return true, nil
	}
	if ak == ASTSequenceOpUnit && bk == ASTSequenceOpUnit {
		return true, nil
	}
	return ak == ASTSequenceOpEmpty && bk == ASTSequenceOpUnit || ak == ASTSequenceOpUnit && bk == ASTSequenceOpEmpty, nil
}

func (m *ASTManager) mkSequenceEmpty(sort astNodeRef) (astNodeRef, error) {
	p := m.sequencePlugin
	if p == nil || m.nodeAt(sort.slot).kind != ASTSort {
		return astNodeRef{}, ErrASTHandle
	}
	family, kind, err := m.declIdentity(sort)
	if err != nil || family != p.family || kind != 0 {
		return astNodeRef{}, ErrASTDomain
	}
	if sort == p.stringSort {
		return m.mkSequenceString("")
	}
	name := p.signatures[ASTSequenceOpEmpty].name
	parameter := astParameterRecord{Kind: ASTParameterAST, ast: sort}
	if err = m.beginFormatInfo(false, p.family, ASTSequenceOpEmpty, &parameter); err != nil {
		return astNodeRef{}, err
	}
	decl, err := m.mkFuncDecl(name, nil, m.external(sort), ASTFuncDeclInfo{m.origin, math.MaxUint32 - 1, m.formatLexical.epoch})
	m.endFormatInfo()
	if err != nil {
		return astNodeRef{}, err
	}
	return m.mkApp(decl, nil)
}

func (m *ASTManager) sequenceSomeValue(sort astNodeRef) (ASTHandle, error) {
	p := m.sequencePlugin
	if p == nil {
		return ASTHandle{}, ErrASTHandle
	}
	family, kind, err := m.declIdentity(sort)
	if err != nil || family != p.family {
		return ASTHandle{}, ErrASTDomain
	}
	if kind == 0 {
		empty, err := m.mkSequenceEmpty(sort)
		return m.external(empty), err
	}
	if kind != 1 {
		return ASTHandle{}, ErrASTDomain
	}
	info, err := m.sortInfo(m.sortAt(sort.slot).info)
	if err != nil || info.count != 1 {
		return ASTHandle{}, ErrASTDomain
	}
	token, err := m.infoParameter(info, 0)
	if err != nil {
		return ASTHandle{}, err
	}
	parameter, err := m.parameter(token)
	if err != nil || parameter.Kind != ASTParameterAST {
		return ASTHandle{}, ErrASTDomain
	}
	empty, err := m.mkSequenceEmpty(parameter.ast)
	if err != nil {
		return ASTHandle{}, err
	}
	name := p.signatures[ASTSequenceOpToRegex].name
	if parameter.ast == p.stringSort {
		name = p.signatures[36].name
	}
	if err = m.beginFormatInfo(false, p.family, ASTSequenceOpToRegex, nil); err != nil {
		return ASTHandle{}, err
	}
	decl, err := m.mkFuncDecl(name, []ASTHandle{m.external(parameter.ast)}, m.external(sort), ASTFuncDeclInfo{m.origin, math.MaxUint32 - 1, m.formatLexical.epoch})
	m.endFormatInfo()
	if err != nil {
		return ASTHandle{}, err
	}
	app, err := m.mkApp(decl, []ASTHandle{m.external(empty)})
	return m.external(app), err
}
