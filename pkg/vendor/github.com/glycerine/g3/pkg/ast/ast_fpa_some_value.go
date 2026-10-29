package ast

// Source: src/ast/fpa_decl_plugin.cpp:856-870; src/util/mpf.h.

// floatingPointSomeValue follows the source get_some_value sequence: read the
// format, construct one MPF NaN, register its numeral, then delete that MPF.
// The existing numeral frame retains a stopped deletion for AST owner cleanup.
func (m *ASTManager) floatingPointSomeValue(sort astNodeRef) (out astNodeRef, result error) {
	p := m.floatingPointPlugin
	if p == nil || p.self != p || !p.published || p.sourceEnded {
		return out, ErrASTHandle
	}
	f, e := p.beginNumeralProbe()
	if e != nil {
		return out, e
	}
	defer p.finishNumeralConstruction(f, &result, &out)
	info, e := m.sortInfo(m.sortAt(sort.slot).info)
	if e != nil {
		return out, e
	}
	eParam, e := m.infoParameter(info, 0)
	if e != nil {
		return out, e
	}
	eWidth, e := m.parameterOfKind(eParam, ASTParameterInt)
	if e != nil {
		return out, e
	}
	sParam, e := m.infoParameter(info, 1)
	if e != nil {
		return out, e
	}
	sWidth, e := m.parameterOfKind(sParam, ASTParameterInt)
	if e != nil {
		return out, e
	}
	if e = p.numeric.MkNaN(uint32(eWidth.integer), uint32(sWidth.integer), &f.value, &f.copy); e != nil {
		return out, e
	}
	return p.mkNumeral(&f.value, nil)
}
