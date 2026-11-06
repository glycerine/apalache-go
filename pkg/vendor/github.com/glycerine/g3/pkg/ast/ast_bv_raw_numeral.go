package ast

// Source: src/ast/bv_decl_plugin.cpp:416-435,475-477,590-592.

func (m *ASTManager) bitVectorNumeralDecl(ps []ASTParameter, arity int) (decl astNodeRef, result error) {
	if len(ps) != 2 || arity != 0 {
		return decl, bvDeclarationError("invalid bit-vector numeral declaration")
	}
	value, e := m.parameter(ps[0])
	if e != nil {
		return decl, e
	}
	if value.Kind != ASTParameterRational {
		return decl, bvDeclarationError("invalid bit-vector numeral declaration")
	}
	width, e := m.parameter(ps[1])
	if e != nil {
		return decl, e
	}
	if width.Kind != ASTParameterInt {
		return decl, bvDeclarationError("invalid bit-vector numeral declaration")
	}
	bits := uint32(width.integer)
	if bits == 0 {
		return decl, bvDeclarationError("bit-vector size must be greater than zero")
	}
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	f, e := m.beginBVNumeralFrame()
	if e != nil {
		return decl, e
	}
	defer f.finishNumeral(&result, nil)
	return m.bitVectorNumeralDeclCore(f, &value.Rational, bits)
}
