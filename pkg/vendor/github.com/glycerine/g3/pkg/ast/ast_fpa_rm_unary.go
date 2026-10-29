package ast

// Source: src/ast/rewriter/fpa_rewriter.cpp:433-466;
// src/util/scoped_numeral.h:24-33.

// RewriteFPARMUnaryNumeral reads the rounding constant before constructing
// the operand and result scoped MPF values. A symbolic argument is refused.
func (m *ASTManager) RewriteFPARMUnaryNumeral(rm, value ASTHandle, kind int32) (out ASTHandle, found bool, result error) {
	if kind != FPAOpSqrt && kind != FPAOpRoundToIntegral {
		return ASTHandle{}, false, ErrASTDomain
	}
	if err := m.lock(); err != nil {
		return ASTHandle{}, false, err
	}
	defer m.mu.Unlock()
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
	hasMode, mode, err := m.readFPARoundingMode(rmRef)
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
	v, err := p.beginNumeralProbe()
	if err != nil {
		return ASTHandle{}, false, err
	}
	defer p.finishNumeralProbe(v, &result, &found)
	found, result = p.isNumeral(valueRef, &v.value, &v.copy)
	if result != nil || !found {
		return ASTHandle{}, found, result
	}
	t, err := p.beginNumeralProbe()
	if err != nil {
		return ASTHandle{}, true, err
	}
	defer p.finishNumeralProbe(t, &result, &found)
	if kind == FPAOpSqrt {
		result = p.numeric.Sqrt(mode, &v.value, &t.value)
	} else {
		result = p.numeric.RoundToIntegral(mode, &v.value, &t.value)
	}
	if result != nil {
		return ASTHandle{}, true, result
	}
	node, result = p.mkNumeral(&t.value, nil)
	return ASTHandle{}, true, result
}
