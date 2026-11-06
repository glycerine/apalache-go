package ast

// Source: src/ast/array_decl_plugin.cpp:543-558;
// src/ast/ast.cpp:2566-2593.

import "errors"

func (m *ASTManager) arraySomeValue(sort astNodeRef) (result ASTHandle, err error) {
	info, e := m.arrayMapSort(m.external(sort))
	if e != nil {
		return result, e
	}
	rangeSort, e := m.arraySortParameter(info, info.count-1)
	if e != nil {
		return result, e
	}
	// Preserve the same scoped callback at each recursive manager invocation.
	// Domains are not inspected while constructing the source constant value.
	value, e := m.getSomeValue(rangeSort)
	if e != nil {
		return result, e
	}
	p, e := m.newParameterAST(m.external(sort))
	if e != nil {
		return result, e
	}
	defer func() { err = errors.Join(err, m.dropParameter(p)) }()
	r, e := m.mkFamilyApp(m.arrayPlugin.family, ASTArrayOpConst, []ASTParameter{p}, []ASTHandle{value})
	return m.external(r), e
}
func (m *ASTManager) arrayFullyInterpreted(sort astNodeRef) (bool, error) {
	info, e := m.arrayMapSort(m.external(sort))
	if e != nil {
		return false, e
	}
	for i := uint32(0); i+1 < info.count; i++ {
		domain, e := m.arraySortParameter(info, i)
		if e != nil {
			return false, e
		}
		interpreted, e := m.isFullyInterpreted(domain)
		if e != nil || !interpreted {
			return false, e
		}
	}
	rangeSort, e := m.arraySortParameter(info, info.count-1)
	if e != nil {
		return false, e
	}
	return m.isFullyInterpreted(rangeSort)
}
