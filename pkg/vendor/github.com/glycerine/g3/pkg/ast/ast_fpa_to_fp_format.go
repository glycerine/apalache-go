package ast

// Source: src/ast/rewriter/fpa_rewriter.cpp:106-119,166-175.

// RewriteFPAToFPFormatNumeral implements the rounding-mode plus FP overload
// of to_fp. The outer MPF exists before the rounding-mode/numeral guards; the
// target-format MPF exists only after the input numeral matches.
func (m *ASTManager) RewriteFPAToFPFormatNumeral(decl, rm, value ASTHandle) (out ASTHandle, found bool, result error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, false, err
	}
	defer m.mu.Unlock()
	d, err := m.requireKind(decl, ASTFuncDecl)
	if err != nil {
		return ASTHandle{}, false, err
	}
	rmRef, err := m.requireExpr(rm)
	if err != nil {
		return ASTHandle{}, false, err
	}
	valueRef, err := m.requireExpr(value)
	if err != nil {
		return ASTHandle{}, false, err
	}
	p := m.floatingPointPlugin
	if p == nil || p.self != p || !p.published || p.sourceEnded {
		return ASTHandle{}, false, ErrASTHandle
	}
	family, kind, err := m.declIdentity(d)
	if err != nil {
		return ASTHandle{}, false, err
	}
	if family != p.family || kind != astFPAToFP {
		return ASTHandle{}, false, ErrASTDomain
	}
	domain, err := m.readInternal(m.declAt(d.slot).domain, 1)
	if err != nil {
		return ASTHandle{}, false, err
	}
	sortInfo, err := m.sortInfo(m.sortAt(domain.slot).info)
	if err != nil {
		return ASTHandle{}, false, err
	}
	if sortInfo.family != p.family || sortInfo.kind != 0 {
		return ASTHandle{}, false, ErrASTDomain
	}
	info, err := m.funcInfo(m.declAt(d.slot).info)
	if err != nil {
		return ASTHandle{}, false, err
	}
	first, err := m.infoParameter(info, 0)
	if err != nil {
		return ASTHandle{}, false, err
	}
	second, err := m.infoParameter(info, 1)
	if err != nil {
		return ASTHandle{}, false, err
	}
	eParam, err := m.parameterOfKind(first, ASTParameterInt)
	if err != nil {
		return ASTHandle{}, false, err
	}
	sParam, err := m.parameterOfKind(second, ASTParameterInt)
	if err != nil {
		return ASTHandle{}, false, err
	}
	ebits, sbits := uint32(eParam.integer), uint32(sParam.integer)
	var node astNodeRef
	defer func() {
		if result != nil {
			found = false
			node = astNodeRef{}
		}
		out = m.external(node)
	}()
	v, err := p.beginNumeralProbe()
	if err != nil {
		return ASTHandle{}, false, err
	}
	defer p.finishNumeralProbe(v, &result, &found)
	hasMode, mode, err := m.readFPARoundingMode(rmRef)
	if err != nil || !hasMode {
		return ASTHandle{}, false, err
	}
	found, result = p.isNumeral(valueRef, &v.value, &v.copy)
	if result != nil || !found {
		return ASTHandle{}, found, result
	}
	if result = m.traceFPAToFPValue(168, "v: ", &v.value); result != nil {
		return ASTHandle{}, true, result
	}
	vf, err := p.beginNumeralProbe()
	if err != nil {
		return ASTHandle{}, true, err
	}
	defer p.finishNumeralProbe(vf, &result, &found)
	result = p.numeric.SetFormat(&vf.value, ebits, sbits, mode, &v.value)
	if result != nil {
		return ASTHandle{}, true, result
	}
	node, result = p.mkNumeral(&vf.value, nil)
	return ASTHandle{}, true, result
}
