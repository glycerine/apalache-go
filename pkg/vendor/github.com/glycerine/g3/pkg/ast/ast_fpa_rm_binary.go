package ast

// Source: src/ast/rewriter/fpa_rewriter.cpp:256-307;
// src/util/scoped_numeral.h:24-33.

// RewriteFPARMBinaryNumerals covers the numeric add, mul and div branches.
// Subtraction is a structural add-of-negation rewrite in the source.
func (m *ASTManager) RewriteFPARMBinaryNumerals(rm, left, right ASTHandle, kind int32) (out ASTHandle, found bool, result error) {
	if kind != FPAOpAdd && kind != FPAOpMul && kind != FPAOpDiv {
		return ASTHandle{}, false, ErrASTDomain
	}
	if err := m.lock(); err != nil {
		return ASTHandle{}, false, err
	}
	defer m.mu.Unlock()
	refs := [3]astNodeRef{}
	for i, h := range [...]ASTHandle{rm, left, right} {
		ref, err := m.requireExpr(h)
		if err != nil {
			return ASTHandle{}, false, err
		}
		refs[i] = ref
	}
	p := m.floatingPointPlugin
	if p == nil || p.self != p || !p.published || p.sourceEnded {
		return ASTHandle{}, false, ErrASTHandle
	}
	hasMode, mode, err := m.readFPARoundingMode(refs[0])
	if err != nil || !hasMode {
		return ASTHandle{}, false, err
	}
	var node astNodeRef
	defer func() {
		if result != nil {
			found = false
			node = astNodeRef{}
		}
		out = m.external(node)
	}()
	v2, err := p.beginNumeralProbe()
	if err != nil {
		return ASTHandle{}, false, err
	}
	defer p.finishNumeralProbe(v2, &result, &found)
	v3, err := p.beginNumeralProbe()
	if err != nil {
		return ASTHandle{}, false, err
	}
	defer p.finishNumeralProbe(v3, &result, &found)
	found, result = p.isNumeral(refs[1], &v2.value, &v2.copy)
	if result != nil || !found {
		return ASTHandle{}, found, result
	}
	found, result = p.isNumeral(refs[2], &v3.value, &v3.copy)
	if result != nil || !found {
		return ASTHandle{}, found, result
	}
	t, err := p.beginNumeralProbe()
	if err != nil {
		return ASTHandle{}, true, err
	}
	defer p.finishNumeralProbe(t, &result, &found)
	switch kind {
	case FPAOpAdd:
		result = p.numeric.Add(mode, &v2.value, &v3.value, &t.value)
	case FPAOpMul:
		result = p.numeric.Mul(mode, &v2.value, &v3.value, &t.value)
	case FPAOpDiv:
		result = p.numeric.Div(mode, &v2.value, &v3.value, &t.value)
	}
	if result != nil {
		return ASTHandle{}, true, result
	}
	node, result = p.mkNumeral(&t.value, nil)
	return ASTHandle{}, true, result
}
