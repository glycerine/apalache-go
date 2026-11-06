package ast

// Source: src/ast/fpa_decl_plugin.cpp:950-957;
// src/ast/rewriter/fpa_rewriter.cpp:476-481.

// MkFPANaNOfSort is the concrete fpa_util::mk_nan(sort*) call used by the
// infinity comparison rewrites. The supplied sort selects its two widths.
func (m *ASTManager) MkFPANaNOfSort(sort ASTHandle) (ASTHandle, error) {
	return m.mkFPASpecialOfSort(sort, astFPANaN)
}

// MkFPAInfinityOfSort is the concrete fpa_util::mk_pinf/mk_ninf(sort*) call.
func (m *ASTManager) MkFPAInfinityOfSort(sort ASTHandle, negative bool) (ASTHandle, error) {
	if negative {
		return m.mkFPASpecialOfSort(sort, astFPAMinusInf)
	}
	return m.mkFPASpecialOfSort(sort, astFPAPlusInf)
}

// MkFPAZeroOfSort is the concrete fpa_util::mk_pzero/mk_nzero(sort*) call.
func (m *ASTManager) MkFPAZeroOfSort(sort ASTHandle, negative bool) (ASTHandle, error) {
	if negative {
		return m.mkFPASpecialOfSort(sort, astFPAMinusZero)
	}
	return m.mkFPASpecialOfSort(sort, astFPAPlusZero)
}

func (m *ASTManager) mkFPASpecialOfSort(sort ASTHandle, kind int32) (ASTHandle, error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, err
	}
	defer m.mu.Unlock()
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	ref, err := m.requireKind(sort, ASTSort)
	if err != nil {
		return ASTHandle{}, err
	}
	p := m.floatingPointPlugin
	if p == nil || !p.published || p.sourceEnded {
		return ASTHandle{}, ErrASTHandle
	}
	if kind == astFPAPlusZero || kind == astFPAMinusZero {
		node, err := p.mkSignedZeroFromSort(ref, kind == astFPAMinusZero)
		return m.external(node), err
	}
	ebits, err := m.fpaUtilExponentBits(ref)
	if err != nil {
		return ASTHandle{}, err
	}
	sbits, err := m.fpaUtilSignificandBits(ref)
	if err != nil {
		return ASTHandle{}, err
	}
	class := astFloatNaN
	switch kind {
	case astFPANaN:
	case astFPAPlusInf:
		class = astFloatPInf
	case astFPAMinusInf:
		class = astFloatNInf
	default:
		return ASTHandle{}, ErrASTDomain
	}
	node, err := p.mkSpecialNumeral(ebits, sbits, class)
	return m.external(node), err
}
