package ast

// Source: src/ast/rewriter/fpa_rewriter.cpp:648-667;
// src/util/scoped_numeral.h:24-33.

const (
	fpaSMTEqMode  int32 = 0
	fpaIEEEEqMode int32 = 1
)

// EqualFPANumerals evaluates the retained SMT equality branch over two
// numeral operands. It constructs both scoped_mpf locals before probing either
// operand, then destroys them in reverse construction order.
func (m *ASTManager) EqualFPANumerals(left, right ASTHandle) (found, equal bool, result error) {
	return m.compareFPANumerals(left, right, fpaSMTEqMode)
}

// EqualFPAIEEENumerals is the distinct fp.eq path: NaNs compare unequal and
// opposite signed zeros compare equal, as in mpf_manager::eq.
func (m *ASTManager) EqualFPAIEEENumerals(left, right ASTHandle) (found, equal bool, result error) {
	return m.compareFPANumerals(left, right, fpaIEEEEqMode)
}

// CompareFPANumerals is the reached numeric branch of fp.lt and fp.leq.
// The source handles NaN/infinity special rewrites before calling this branch.
func (m *ASTManager) CompareFPANumerals(left, right ASTHandle, kind int32) (found, comparison bool, result error) {
	if kind != FPAOpLt && kind != FPAOpLe {
		return false, false, ErrASTDomain
	}
	return m.compareFPANumerals(left, right, kind)
}

func (m *ASTManager) compareFPANumerals(left, right ASTHandle, mode int32) (found, equal bool, result error) {
	if err := m.lock(); err != nil {
		return false, false, err
	}
	defer m.mu.Unlock()
	a, err := m.requireExpr(left)
	if err != nil {
		return false, false, err
	}
	b, err := m.requireExpr(right)
	if err != nil {
		return false, false, err
	}
	p := m.floatingPointPlugin
	if p == nil || p.self != p || !p.published || p.sourceEnded {
		return false, false, ErrASTHandle
	}
	v1, err := p.beginNumeralProbe()
	if err != nil {
		return false, false, err
	}
	defer p.finishNumeralProbe(v1, &result, &found)
	v2, err := p.beginNumeralProbe()
	if err != nil {
		return false, false, err
	}
	defer p.finishNumeralProbe(v2, &result, &found)
	found, result = p.isNumeral(a, &v1.value, &v1.copy)
	if result != nil || !found {
		return found, false, result
	}
	found, result = p.isNumeral(b, &v2.value, &v2.copy)
	if result != nil || !found {
		return found, false, result
	}
	switch mode {
	case FPAOpLt:
		equal, result = p.numeric.Lt(&v1.value, &v2.value)
		return true, equal, result
	case FPAOpLe:
		equal, result = p.numeric.Lte(&v1.value, &v2.value)
		return true, equal, result
	case fpaIEEEEqMode:
		equal, result = p.numeric.Eq(&v1.value, &v2.value)
		return true, equal, result
	}
	firstNaN, err := p.numeric.IsNaN(&v1.value)
	if err != nil {
		return true, false, err
	}
	if firstNaN {
		secondNaN, err := p.numeric.IsNaN(&v2.value)
		if err != nil || secondNaN {
			return true, secondNaN, err
		}
	}
	firstZero, err := p.numeric.IsZero(&v1.value)
	if err != nil {
		return true, false, err
	}
	if firstZero {
		secondZero, err := p.numeric.IsZero(&v2.value)
		if err != nil {
			return true, false, err
		}
		if secondZero && v1.value.Sign() != v2.value.Sign() {
			return true, false, nil
		}
	}
	equal, result = p.numeric.EqCore(&v1.value, &v2.value)
	return true, equal, result
}
