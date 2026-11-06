package ast

// Source: src/ast/rewriter/fpa_rewriter.cpp:369-415;
// src/util/scoped_numeral.h:24-33.

// RewriteFPAExtremumNumerals evaluates the numeric branch after the rewriter's
// NaN guards. Opposite signed zeros deliberately remain unreduced.
func (m *ASTManager) RewriteFPAExtremumNumerals(left, right ASTHandle, kind int32) (out ASTHandle, found bool, result error) {
	if kind != FPAOpMin && kind != FPAOpMax {
		return ASTHandle{}, false, ErrASTDomain
	}
	if err := m.lock(); err != nil {
		return ASTHandle{}, false, err
	}
	defer m.mu.Unlock()
	a, err := m.requireExpr(left)
	if err != nil {
		return ASTHandle{}, false, err
	}
	b, err := m.requireExpr(right)
	if err != nil {
		return ASTHandle{}, false, err
	}
	p := m.floatingPointPlugin
	if p == nil || p.self != p || !p.published || p.sourceEnded {
		return ASTHandle{}, false, ErrASTHandle
	}
	var node astNodeRef
	defer func() {
		if result != nil {
			found = false
			node = astNodeRef{}
		}
		out = m.external(node)
	}()
	v1, err := p.beginNumeralProbe()
	if err != nil {
		return ASTHandle{}, false, err
	}
	defer p.finishNumeralProbe(v1, &result, &found)
	v2, err := p.beginNumeralProbe()
	if err != nil {
		return ASTHandle{}, false, err
	}
	defer p.finishNumeralProbe(v2, &result, &found)
	found, result = p.isNumeral(a, &v1.value, &v1.copy)
	if result != nil || !found {
		return ASTHandle{}, found, result
	}
	found, result = p.isNumeral(b, &v2.value, &v2.copy)
	if result != nil || !found {
		return ASTHandle{}, found, result
	}
	zero1, err := p.numeric.IsZero(&v1.value)
	if err != nil {
		return ASTHandle{}, true, err
	}
	if zero1 {
		zero2, err := p.numeric.IsZero(&v2.value)
		if err != nil {
			return ASTHandle{}, true, err
		}
		if zero2 && v1.value.Sign() != v2.value.Sign() {
			return ASTHandle{}, false, nil
		}
	}
	t, err := p.beginNumeralProbe()
	if err != nil {
		return ASTHandle{}, true, err
	}
	defer p.finishNumeralProbe(t, &result, &found)
	if kind == FPAOpMin {
		result = p.numeric.Minimum(&v1.value, &v2.value, &t.value)
	} else {
		result = p.numeric.Maximum(&v1.value, &v2.value, &t.value)
	}
	if result != nil {
		return ASTHandle{}, true, result
	}
	node, result = p.mkNumeral(&t.value, nil)
	return ASTHandle{}, true, result
}
