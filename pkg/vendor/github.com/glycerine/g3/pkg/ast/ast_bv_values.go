package ast

// Source: src/ast/bv_decl_plugin.cpp:632-634,746-751.

func (m *ASTManager) bitVectorSomeValue(sort astNodeRef) (ASTHandle, error) {
	// Numerical failure transport must not reenter the locked AST owner.
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	info, e := m.sortInfo(m.sortAt(sort.slot).info)
	if e != nil {
		return ASTHandle{}, e
	}
	p, e := m.infoParameter(info, 0)
	if e != nil {
		return ASTHandle{}, e
	}
	width, e := m.parameter(p)
	if e != nil {
		return ASTHandle{}, e
	}
	if width.Kind != ASTParameterInt {
		return ASTHandle{}, ErrASTDomain
	}
	r, e := m.mkBitVectorNumeralInput(nil, uint32(width.integer), nil, true)
	return m.external(r), e
}
