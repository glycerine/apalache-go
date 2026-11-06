package ast

// Source: src/ast/rewriter/fpa_rewriter.cpp:308-370;
// src/util/scoped_numeral.h:24-33.

// RewriteFPAUnaryNumeral owns one caller-local scoped_mpf through the source
// neg/abs numeric branch. The result becomes visible only after destruction.
func (m *ASTManager) RewriteFPAUnaryNumeral(h ASTHandle, kind int32) (out ASTHandle, found bool, result error) {
	if kind != FPAOpNeg && kind != FPAOpAbs {
		return ASTHandle{}, false, ErrASTDomain
	}
	if err := m.lock(); err != nil {
		return ASTHandle{}, false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return ASTHandle{}, false, err
	}
	p := m.floatingPointPlugin
	if p == nil || p.self != p || !p.published || p.sourceEnded {
		return ASTHandle{}, false, ErrASTHandle
	}
	f, err := p.beginNumeralProbe()
	if err != nil {
		return ASTHandle{}, false, err
	}
	var node astNodeRef
	defer func() {
		p.finishNumeralConstruction(f, &result, &node)
		if result != nil {
			found = false
		}
		out = m.external(node)
	}()
	found, result = p.isNumeral(r, &f.value, &f.copy)
	if result != nil || !found {
		return ASTHandle{}, found, result
	}
	if kind == FPAOpNeg {
		result = p.numeric.Neg(&f.value)
	} else {
		var negative bool
		negative, result = p.numeric.IsNegative(&f.value)
		if result == nil && negative {
			result = p.numeric.Neg(&f.value)
		}
	}
	if result != nil {
		return ASTHandle{}, true, result
	}
	node, result = p.mkNumeral(&f.value, nil)
	return ASTHandle{}, true, result
}
