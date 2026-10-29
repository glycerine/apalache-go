package ast

// Source: src/ast/rewriter/fpa_rewriter.cpp:794-813;
// src/util/scoped_numeral.h:24-33.

import "errors"

// RewriteFPAToRealNumeral follows the source scoped MPF and rational branch.
// A special value is unspecified unless the hardware interpretation is set.
func (m *ASTManager) RewriteFPAToRealNumeral(h ASTHandle, highUnspecified bool) (out ASTHandle, found bool, result error) {
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
	found, result = p.isNumeral(r, &v.value, &v.copy)
	if result != nil || !found {
		return ASTHandle{}, found, result
	}
	nan, err := p.numeric.IsNaN(&v.value)
	if err != nil {
		return ASTHandle{}, true, err
	}
	inf := false
	if !nan {
		inf, err = p.numeric.IsInf(&v.value)
		if err != nil {
			return ASTHandle{}, true, err
		}
	}
	if nan || inf {
		if !highUnspecified {
			return ASTHandle{}, false, nil
		}
		node, result = m.mkArithmeticNumeral(nil, false, true)
		return ASTHandle{}, true, result
	}
	q, err := m.numerals.NewValue()
	if err != nil {
		return ASTHandle{}, true, err
	}
	defer func() {
		result = errors.Join(result, m.numerals.Del(&q))
	}()
	result = p.numeric.ToRational(&v.value, m.numerals.Manager, &q.Raw)
	if result != nil {
		return ASTHandle{}, true, result
	}
	node, result = m.mkArithmeticNumeral(&q, false, false)
	return ASTHandle{}, true, result
}
