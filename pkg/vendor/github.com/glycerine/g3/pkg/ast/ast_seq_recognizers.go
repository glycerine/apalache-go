package ast

// Source: src/ast/seq_decl_plugin.h:198-225,246-288.

func (m *ASTManager) sequenceSortParameter(sort astNodeRef, kind int32) (astNodeRef, bool, error) {
	p := m.sequencePlugin
	if p == nil || m.nodeAt(sort.slot).kind != ASTSort {
		return astNodeRef{}, false, nil
	}
	family, actualKind, err := m.declIdentity(sort)
	if err != nil || family != p.family || actualKind != kind {
		return astNodeRef{}, false, err
	}
	info, err := m.sortInfo(m.sortAt(sort.slot).info)
	if err != nil || info.count != 1 {
		return astNodeRef{}, false, err
	}
	token, err := m.infoParameter(info, 0)
	if err != nil {
		return astNodeRef{}, false, err
	}
	parameter, err := m.parameter(token)
	if err != nil || parameter.Kind != ASTParameterAST {
		if err == nil {
			err = ErrASTDomain
		}
		return astNodeRef{}, false, err
	}
	return parameter.ast, true, nil
}

func (m *ASTManager) SequenceSortElement(sort ASTHandle) (ASTHandle, bool, error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(sort, ASTSort)
	if err != nil {
		return ASTHandle{}, false, err
	}
	element, matched, err := m.sequenceSortParameter(r, 0)
	return m.external(element), matched, err
}

func (m *ASTManager) RegexSequenceSort(sort ASTHandle) (ASTHandle, bool, error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(sort, ASTSort)
	if err != nil {
		return ASTHandle{}, false, err
	}
	sequence, matched, err := m.sequenceSortParameter(r, 1)
	return m.external(sequence), matched, err
}

func (m *ASTManager) IsStringSort(sort ASTHandle) (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(sort, ASTSort)
	if err != nil {
		return false, err
	}
	element, matched, err := m.sequenceSortParameter(r, 0)
	return matched && element == m.sequencePlugin.char, err
}

func (m *ASTManager) IsCharacterSort(sort ASTHandle) (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(sort, ASTSort)
	if err != nil {
		return false, err
	}
	return m.sequencePlugin != nil && r == m.sequencePlugin.char, nil
}

func (m *ASTManager) IsSequenceOperation(h ASTHandle, kind int32) (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return false, err
	}
	if m.nodeAt(r.slot).kind != ASTApp || m.sequencePlugin == nil {
		return false, nil
	}
	family, actualKind, err := m.declIdentity(m.appAt(r.slot).decl)
	return family == m.sequencePlugin.family && actualKind == kind, err
}

func (m *ASTManager) IsSequenceEmpty(h ASTHandle) (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return false, err
	}
	return m.isSequenceEmptyRef(r)
}

func (m *ASTManager) isSequenceEmptyRef(r astNodeRef) (bool, error) {
	if m.nodeAt(r.slot).kind != ASTApp || m.sequencePlugin == nil {
		return false, nil
	}
	decl := m.appAt(r.slot).decl
	family, kind, err := m.declIdentity(decl)
	if err != nil || family != m.sequencePlugin.family {
		return false, err
	}
	if kind == ASTSequenceOpEmpty {
		return true, nil
	}
	if kind != ASTSequenceOpStringConst {
		return false, nil
	}
	info, err := m.funcInfo(m.declAt(decl.slot).info)
	if err != nil || info.count != 1 {
		return false, err
	}
	token, err := m.infoParameter(info, 0)
	if err != nil {
		return false, err
	}
	parameter, err := m.parameter(token)
	if err != nil || parameter.Kind != ASTParameterSymbol {
		return false, err
	}
	text, err := m.Symbols.String(parameter.symbol)
	return text == "", err
}

func (m *ASTManager) IsSequenceUnit(h ASTHandle) (bool, error) {
	return m.IsSequenceOperation(h, ASTSequenceOpUnit)
}
func (m *ASTManager) IsSequenceConcat(h ASTHandle) (bool, error) {
	return m.IsSequenceOperation(h, ASTSequenceOpConcat)
}
func (m *ASTManager) IsSequenceSkolem(h ASTHandle) (bool, error) {
	return m.IsSequenceOperation(h, 42)
}
