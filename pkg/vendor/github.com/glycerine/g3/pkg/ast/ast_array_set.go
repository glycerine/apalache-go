package ast

// Source: src/ast/array_decl_plugin.cpp:329-431;
// src/ast/array_decl_plugin.h:176-185.

import "errors"

func (m *ASTManager) arraySetDecl(kind int32, domain []ASTHandle) (result astNodeRef, err error) {
	n := len(domain)
	switch kind {
	case ASTArrayOpSetUnion, ASTArrayOpSetIntersect:
		if n == 0 {
			return result, ErrASTDomain
		}
	case ASTArrayOpSetDifference, ASTArrayOpSetSubset:
		if n != 2 {
			return result, ErrASTDomain
		}
	case ASTArrayOpSetComplement:
		if n != 1 {
			return result, ErrASTDomain
		}
	default:
		return result, ErrASTDomain
	}
	var first astNodeRef
	for i, h := range domain {
		s, e := m.requireKind(h, ASTSort)
		if e != nil {
			return result, e
		}
		if i == 0 {
			first = s
		} else if s != first {
			return result, ErrASTDomain
		}
		fid, _, e := m.declIdentity(s)
		if e != nil {
			return result, e
		}
		if fid != m.arrayPlugin.family {
			return result, ErrASTDomain
		}
	}
	info, e := m.sortInfo(m.sortAt(first.slot).info)
	if e != nil {
		return result, e
	}
	if info.count <= 1 {
		return result, ErrASTDomain
	}
	last, e := m.arraySortParameter(info, info.count-1)
	if e != nil {
		return result, e
	}
	if last != m.boolSort {
		return result, ErrASTDomain
	}
	nameIndex := kind - ASTArrayOpSetUnion + 5
	resultSort := first
	if kind == ASTArrayOpSetSubset {
		resultSort = m.boolSort
	}
	if kind == ASTArrayOpSetUnion || kind == ASTArrayOpSetIntersect {
		flags := ASTFuncFlags{LeftAssociative: true, RightAssociative: true, Commutative: true, Idempotent: true}
		normalized := []ASTHandle{domain[0], domain[0]}
		var ps []ASTParameter
		if kind == ASTArrayOpSetUnion {
			p, e := m.newParameterAST(domain[0])
			if e != nil {
				return result, e
			}
			defer func() { err = errors.Join(err, m.dropParameter(p)) }()
			ps = []ASTParameter{p}
		}
		return m.pluginDecl(m.arrayPlugin.names[nameIndex], m.arrayPlugin.family, kind, ps, normalized, resultSort, flags, false)
	}
	return m.pluginDecl(m.arrayPlugin.names[nameIndex], m.arrayPlugin.family, kind, nil, domain, resultSort, ASTFuncFlags{}, false)
}

func (m *ASTManager) mkConstArray(sort, value ASTHandle) (result ASTHandle, err error) {
	name, e := m.Symbols.Intern("array")
	if e != nil {
		return result, e
	}
	fid, e := m.families.MkID(name)
	if e != nil {
		return result, e
	}
	p, e := m.newParameterAST(sort)
	if e != nil {
		return result, e
	}
	defer func() { err = errors.Join(err, m.dropParameter(p)) }()
	r, e := m.mkFamilyApp(fid, ASTArrayOpConst, []ASTParameter{p}, []ASTHandle{value})
	return m.external(r), e
}
func (m *ASTManager) MkConstArray(sort, value ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.mkConstArray(sort, value)
}
func (m *ASTManager) MkEmptySet(sort ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.mkConstArray(sort, m.external(m.falseApp))
}
func (m *ASTManager) MkFullSet(sort ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.mkConstArray(sort, m.external(m.trueApp))
}
