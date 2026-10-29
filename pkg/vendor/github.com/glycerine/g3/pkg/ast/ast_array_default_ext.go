package ast

// Source: src/ast/array_decl_plugin.cpp:211-232,307-325,478-487,598-602.

import "errors"

func (m *ASTManager) arrayDefaultDecl(domain []ASTHandle) (result astNodeRef, err error) {
	if len(domain) != 1 {
		return result, ErrASTDomain
	}
	sort, e := m.requireKind(domain[0], ASTSort)
	if e != nil {
		return result, e
	}
	info, e := m.sortInfo(m.sortAt(sort.slot).info)
	if e != nil {
		return result, e
	}
	if info.count <= 1 {
		return result, ErrASTDomain
	}
	last, e := m.infoParameter(info, info.count-1)
	if e != nil {
		return result, e
	}
	// Source creates the actual lexical parameter copy before checking its kind.
	p, e := m.copyParameter(last)
	if e != nil {
		return result, e
	}
	defer func() { err = errors.Join(err, m.dropParameter(p)) }()
	record, e := m.parameter(p)
	if e != nil {
		return result, e
	}
	if record.Kind != ASTParameterAST {
		return result, ErrASTDomain
	}
	rangeSort, e := m.requireKind(m.external(record.ast), ASTSort)
	if e != nil {
		return result, e
	}
	return m.pluginDecl(m.arrayPlugin.names[3], m.arrayPlugin.family, ASTArrayOpDefault, nil, domain, rangeSort, ASTFuncFlags{}, false)
}
func (m *ASTManager) arrayExtDecl(ps []ASTParameter, domain []ASTHandle) (result astNodeRef, err error) {
	index := uint32(0)
	if len(ps) != 0 {
		if len(ps) != 1 {
			return result, ErrASTDomain
		}
		p, e := m.parameter(ps[0])
		if e != nil {
			return result, e
		}
		if p.Kind != ASTParameterInt {
			return result, ErrASTDomain
		}
		index = uint32(p.integer)
	}
	if len(domain) != 2 || domain[0] != domain[1] {
		return result, ErrASTDomain
	}
	sort, e := m.requireKind(domain[0], ASTSort)
	if e != nil {
		return result, e
	}
	info, e := m.sortInfo(m.sortAt(sort.slot).info)
	if e != nil {
		return result, e
	}
	if info.count == 0 || index >= info.count-1 {
		return result, ErrASTDomain
	}
	rangeSort, e := m.arraySortParameter(info, index)
	if e != nil {
		return result, e
	}
	p, e := m.newParameterInt(int32(index))
	if e != nil {
		return result, e
	}
	defer func() { err = errors.Join(err, m.dropParameter(p)) }()
	return m.pluginDecl(m.arrayPlugin.names[10], m.arrayPlugin.family, ASTArrayOpExt, []ASTParameter{p}, domain, rangeSort, ASTFuncFlags{}, false)
}

// MkArrayExtDecl follows array_util::mk_array_ext, returning the declaration
// whose result sort is the selected index domain, not the array value range.
func (m *ASTManager) MkArrayExtDecl(sort ASTHandle, index uint32) (result ASTHandle, err error) {
	if e := m.lock(); e != nil {
		return result, e
	}
	defer m.mu.Unlock()
	name, e := m.Symbols.Intern("array")
	if e != nil {
		return result, e
	}
	fid, e := m.families.MkID(name)
	if e != nil {
		return result, e
	}
	p, e := m.newParameterInt(int32(index))
	if e != nil {
		return result, e
	}
	defer func() { err = errors.Join(err, m.dropParameter(p)) }()
	if !m.pluginAt(fid).present {
		return result, nil
	}
	r, e := m.arrayDecl(ASTArrayOpExt, []ASTParameter{p}, []ASTHandle{sort, sort}, ASTHandle{})
	return m.external(r), e
}
