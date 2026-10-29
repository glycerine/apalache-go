package ast

// Source: src/ast/fpa_decl_plugin.h:310-326.

// FloatingPointSort constructs (_ FloatingPoint ebits sbits).
func (m *ASTManager) FloatingPointSort(ebits, sbits uint32) (ASTHandle, error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, err
	}
	defer m.mu.Unlock()
	if m.floatingPointPlugin == nil || m.floatingPointPlugin.sourceEnded {
		return ASTHandle{}, ErrASTHandle
	}
	r, err := m.mkFloatSort(ebits, sbits)
	return m.external(r), err
}

// RoundingModeSort returns the unique FPA rounding-mode sort.
func (m *ASTManager) RoundingModeSort() (ASTHandle, error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, err
	}
	defer m.mu.Unlock()
	if m.floatingPointPlugin == nil || m.floatingPointPlugin.sourceEnded {
		return ASTHandle{}, ErrASTHandle
	}
	r, err := m.floatingPointSort(1, nil)
	return m.external(r), err
}

func (m *ASTManager) floatingPointSortShape(r astNodeRef) (ebits, sbits uint32, matched bool, err error) {
	if m.floatingPointPlugin == nil || m.floatingPointPlugin.sourceEnded || r.slot == 0 || m.nodeAt(r.slot).kind != ASTSort {
		return 0, 0, false, nil
	}
	info, err := m.sortInfo(m.sortAt(r.slot).info)
	if err != nil {
		return 0, 0, false, err
	}
	if info.family != m.floatingPointPlugin.family || info.kind != 0 || info.count != 2 {
		return 0, 0, false, nil
	}
	first, err := m.infoParameter(info, 0)
	if err != nil {
		return 0, 0, false, err
	}
	second, err := m.infoParameter(info, 1)
	if err != nil {
		return 0, 0, false, err
	}
	e, err := m.parameterOfKind(first, ASTParameterInt)
	if err != nil {
		return 0, 0, false, err
	}
	s, err := m.parameterOfKind(second, ASTParameterInt)
	if err != nil {
		return 0, 0, false, err
	}
	return uint32(e.integer), uint32(s.integer), true, nil
}

// FloatingPointSortShape recognizes a floating-point sort and returns its
// exponent and significand widths.
func (m *ASTManager) FloatingPointSortShape(sort ASTHandle) (ebits, sbits uint32, matched bool, err error) {
	if err = m.lock(); err != nil {
		return 0, 0, false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(sort, ASTSort)
	if err != nil {
		return 0, 0, false, err
	}
	return m.floatingPointSortShape(r)
}

// IsRoundingModeSort recognizes the FPA rounding-mode sort.
func (m *ASTManager) IsRoundingModeSort(sort ASTHandle) (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(sort, ASTSort)
	if err != nil {
		return false, err
	}
	if m.floatingPointPlugin == nil || m.floatingPointPlugin.sourceEnded {
		return false, nil
	}
	info, err := m.sortInfo(m.sortAt(r.slot).info)
	if err != nil {
		return false, err
	}
	return info.family == m.floatingPointPlugin.family && info.kind == 1, nil
}
