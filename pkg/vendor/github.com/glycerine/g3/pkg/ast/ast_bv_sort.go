package ast

// Source: src/ast/bv_decl_plugin.cpp:169-178.

func (m *ASTManager) bitVectorSort(ps []ASTParameter) (astNodeRef, error) {
	if len(ps) != 1 {
		return astNodeRef{}, bvDeclarationError("expecting one integer parameter to bit-vector sort")
	}
	parameter, e := m.parameter(ps[0])
	if e != nil {
		return astNodeRef{}, e
	}
	if parameter.Kind != ASTParameterInt {
		return astNodeRef{}, bvDeclarationError("expecting one integer parameter to bit-vector sort")
	}
	width := uint32(parameter.integer)
	if width == 0 {
		return astNodeRef{}, bvDeclarationError("bit-vector size must be greater than zero")
	}
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	return m.bitVectorPlugin.mkSort(width)
}
