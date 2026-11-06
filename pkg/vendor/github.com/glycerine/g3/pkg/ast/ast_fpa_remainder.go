package ast

// Source: src/ast/rewriter/fpa_rewriter.cpp:340-353;
// src/util/scoped_numeral.h:24-33.

// RewriteFPARemainderNumerals evaluates the reached numeric fp.rem branch.
// The two operand locals precede both numeral probes; the result local is
// constructed only after both probes succeed. All retire in reverse order.
func (m *ASTManager) RewriteFPARemainderNumerals(left, right ASTHandle) (out ASTHandle, found bool, result error) {
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
	t, err := p.beginNumeralProbe()
	if err != nil {
		return ASTHandle{}, true, err
	}
	defer p.finishNumeralProbe(t, &result, &found)
	result = p.numeric.Rem(&v1.value, &v2.value, &t.value)
	if result != nil {
		return ASTHandle{}, true, result
	}
	node, result = p.mkNumeral(&t.value, nil)
	return ASTHandle{}, true, result
}
