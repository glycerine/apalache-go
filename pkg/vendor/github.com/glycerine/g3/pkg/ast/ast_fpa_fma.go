package ast

// Source: src/ast/rewriter/fpa_rewriter.cpp:417-431;
// src/util/scoped_numeral.h:24-33.

// RewriteFPAFMANumerals evaluates the source fused operation after recognizing
// a rounding constant. Three operand values are constructed before probing;
// a fourth result value is constructed only after all three probes succeed.
func (m *ASTManager) RewriteFPAFMANumerals(rm, x, y, z ASTHandle) (out ASTHandle, found bool, result error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, false, err
	}
	defer m.mu.Unlock()
	refs := [4]astNodeRef{}
	for i, h := range [...]ASTHandle{rm, x, y, z} {
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
	v4, err := p.beginNumeralProbe()
	if err != nil {
		return ASTHandle{}, false, err
	}
	defer p.finishNumeralProbe(v4, &result, &found)
	for i, v := range [...]*astFloatNumeralProbe{v2, v3, v4} {
		found, result = p.isNumeral(refs[i+1], &v.value, &v.copy)
		if result != nil || !found {
			return ASTHandle{}, found, result
		}
	}
	t, err := p.beginNumeralProbe()
	if err != nil {
		return ASTHandle{}, true, err
	}
	defer p.finishNumeralProbe(t, &result, &found)
	result = p.numeric.FMA(mode, &v2.value, &v3.value, &v4.value, &t.value)
	if result != nil {
		return ASTHandle{}, true, result
	}
	node, result = p.mkNumeral(&t.value, nil)
	return ASTHandle{}, true, result
}
