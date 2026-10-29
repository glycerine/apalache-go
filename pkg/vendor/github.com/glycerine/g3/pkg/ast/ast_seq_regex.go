package ast

// Source: src/ast/seq_decl_plugin.cpp:620-646,674-690,730-767,795-801.

func (m *ASTManager) regexSortForSequence(sequenceSort astNodeRef) (astNodeRef, error) {
	family, kind, err := m.declIdentity(sequenceSort)
	if err != nil || family != m.sequencePlugin.family || kind != 0 {
		return astNodeRef{}, ErrASTDomain
	}
	parameter := astParameterRecord{Kind: ASTParameterAST, ast: sequenceSort}
	return m.sequenceSort(m.sequencePlugin, 1, &parameter)
}

func (m *ASTManager) regexSequenceSort(regexSort astNodeRef) (astNodeRef, error) {
	family, kind, err := m.declIdentity(regexSort)
	if err != nil || family != m.sequencePlugin.family || kind != 1 {
		return astNodeRef{}, ErrASTDomain
	}
	info, err := m.sortInfo(m.sortAt(regexSort.slot).info)
	if err != nil || info.count != 1 {
		return astNodeRef{}, ErrASTDomain
	}
	token, err := m.infoParameter(info, 0)
	if err != nil {
		return astNodeRef{}, err
	}
	parameter, err := m.parameter(token)
	if err != nil || parameter.Kind != ASTParameterAST {
		return astNodeRef{}, ErrASTDomain
	}
	return parameter.ast, nil
}

func (m *ASTManager) MkSequenceToRegex(sequence ASTHandle) (ASTHandle, error) {
	return m.withSequenceFactory(func() (ASTHandle, error) {
		ref, err := m.requireExpr(sequence)
		if err != nil {
			return ASTHandle{}, err
		}
		sequenceSort, err := m.sortOf(ref)
		if err != nil {
			return ASTHandle{}, err
		}
		family, kind, err := m.declIdentity(sequenceSort)
		if err != nil || family != m.sequencePlugin.family || kind != 0 {
			return ASTHandle{}, ErrASTDomain
		}
		regexSort, err := m.regexSortForSequence(sequenceSort)
		if err != nil {
			return ASTHandle{}, err
		}
		name := "seq.to.re"
		if sequenceSort == m.sequencePlugin.stringSort {
			name = "str.to.re"
		}
		decl, err := m.makePluginDeclaration(m.sequencePlugin.family, name, ASTSequenceOpToRegex, []astNodeRef{sequenceSort}, regexSort, ASTFuncFlags{})
		if err != nil {
			return ASTHandle{}, err
		}
		app, err := m.mkApp(decl, []ASTHandle{sequence})
		return m.external(app), err
	})
}

func (m *ASTManager) MkSequenceInRegex(sequence, regex ASTHandle) (ASTHandle, error) {
	return m.withSequenceFactory(func() (ASTHandle, error) {
		sequenceRef, err := m.requireExpr(sequence)
		if err != nil {
			return ASTHandle{}, err
		}
		sequenceSort, err := m.sortOf(sequenceRef)
		if err != nil {
			return ASTHandle{}, err
		}
		regexRef, err := m.requireExpr(regex)
		if err != nil {
			return ASTHandle{}, err
		}
		regexSort, err := m.sortOf(regexRef)
		if err != nil {
			return ASTHandle{}, err
		}
		expected, err := m.regexSequenceSort(regexSort)
		if err != nil || expected != sequenceSort {
			return ASTHandle{}, ErrASTDomain
		}
		name := "seq.in.re"
		if sequenceSort == m.sequencePlugin.stringSort {
			name = "str.in.re"
		}
		decl, err := m.makePluginDeclaration(m.sequencePlugin.family, name, 12, []astNodeRef{sequenceSort, regexSort}, m.boolSort, ASTFuncFlags{})
		if err != nil {
			return ASTHandle{}, err
		}
		app, err := m.mkApp(decl, []ASTHandle{sequence, regex})
		return m.external(app), err
	})
}

func (m *ASTManager) regexUnary(kind int32, name string, regex ASTHandle) (ASTHandle, error) {
	return m.withSequenceFactory(func() (ASTHandle, error) {
		ref, err := m.requireExpr(regex)
		if err != nil {
			return ASTHandle{}, err
		}
		sort, err := m.sortOf(ref)
		if err != nil {
			return ASTHandle{}, err
		}
		if _, err = m.regexSequenceSort(sort); err != nil {
			return ASTHandle{}, err
		}
		decl, err := m.makePluginDeclaration(m.sequencePlugin.family, name, kind, []astNodeRef{sort}, sort, ASTFuncFlags{})
		if err != nil {
			return ASTHandle{}, err
		}
		app, err := m.mkApp(decl, []ASTHandle{regex})
		return m.external(app), err
	})
}

func (m *ASTManager) MkRegexPlus(regex ASTHandle) (ASTHandle, error) {
	return m.regexUnary(13, "re.+", regex)
}
func (m *ASTManager) MkRegexStar(regex ASTHandle) (ASTHandle, error) {
	return m.regexUnary(14, "re.*", regex)
}
func (m *ASTManager) MkRegexOption(regex ASTHandle) (ASTHandle, error) {
	return m.regexUnary(15, "re.opt", regex)
}
func (m *ASTManager) MkRegexComplement(regex ASTHandle) (ASTHandle, error) {
	return m.regexUnary(21, "re.complement", regex)
}

func (m *ASTManager) MkRegexRange(lower, upper ASTHandle) (ASTHandle, error) {
	return m.withSequenceFactory(func() (ASTHandle, error) {
		lowerRef, err := m.requireExpr(lower)
		if err != nil {
			return ASTHandle{}, err
		}
		lowerSort, err := m.sortOf(lowerRef)
		if err != nil {
			return ASTHandle{}, err
		}
		upperRef, err := m.requireExpr(upper)
		if err != nil {
			return ASTHandle{}, err
		}
		upperSort, err := m.sortOf(upperRef)
		if err != nil || lowerSort != upperSort {
			return ASTHandle{}, ErrASTDomain
		}
		regexSort, err := m.regexSortForSequence(lowerSort)
		if err != nil {
			return ASTHandle{}, err
		}
		decl, err := m.makePluginDeclaration(m.sequencePlugin.family, "re.range", 16, []astNodeRef{lowerSort, lowerSort}, regexSort, ASTFuncFlags{})
		if err != nil {
			return ASTHandle{}, err
		}
		app, err := m.mkApp(decl, []ASTHandle{lower, upper})
		return m.external(app), err
	})
}

func (m *ASTManager) regexBinary(kind int32, name string, left, right ASTHandle) (ASTHandle, error) {
	return m.withSequenceFactory(func() (ASTHandle, error) {
		leftRef, err := m.requireExpr(left)
		if err != nil {
			return ASTHandle{}, err
		}
		leftSort, err := m.sortOf(leftRef)
		if err != nil {
			return ASTHandle{}, err
		}
		rightRef, err := m.requireExpr(right)
		if err != nil {
			return ASTHandle{}, err
		}
		rightSort, err := m.sortOf(rightRef)
		if err != nil || leftSort != rightSort {
			return ASTHandle{}, ErrASTDomain
		}
		if _, err = m.regexSequenceSort(leftSort); err != nil {
			return ASTHandle{}, err
		}
		decl, err := m.makePluginDeclaration(m.sequencePlugin.family, name, kind, []astNodeRef{leftSort, leftSort}, leftSort, ASTFuncFlags{RightAssociative: true})
		if err != nil {
			return ASTHandle{}, err
		}
		app, err := m.mkApp(decl, []ASTHandle{left, right})
		return m.external(app), err
	})
}

func (m *ASTManager) MkRegexConcat(left, right ASTHandle) (ASTHandle, error) {
	return m.regexBinary(17, "re.++", left, right)
}
func (m *ASTManager) MkRegexUnion(left, right ASTHandle) (ASTHandle, error) {
	return m.regexBinary(18, "re.union", left, right)
}
func (m *ASTManager) MkRegexIntersect(left, right ASTHandle) (ASTHandle, error) {
	return m.regexBinary(19, "re.inter", left, right)
}

func (m *ASTManager) regexConstant(kind int32, genericName, stringName string, sort ASTHandle) (ASTHandle, error) {
	return m.withSequenceFactory(func() (ASTHandle, error) {
		r, err := m.requireKind(sort, ASTSort)
		if err != nil {
			return ASTHandle{}, err
		}
		sequenceSort, err := m.regexSequenceSort(r)
		if err != nil {
			return ASTHandle{}, err
		}
		name := genericName
		if sequenceSort == m.sequencePlugin.stringSort {
			name = stringName
		}
		decl, err := m.makePluginDeclaration(m.sequencePlugin.family, name, kind, nil, r, ASTFuncFlags{})
		if err != nil {
			return ASTHandle{}, err
		}
		app, err := m.mkApp(decl, nil)
		return m.external(app), err
	})
}

func (m *ASTManager) MkRegexEmpty(sort ASTHandle) (ASTHandle, error) {
	return m.regexConstant(22, "re.empty", "re.nostr", sort)
}
func (m *ASTManager) MkRegexFull(sort ASTHandle) (ASTHandle, error) {
	return m.regexConstant(23, "re.all", "re.all", sort)
}
func (m *ASTManager) MkRegexFullChar(sort ASTHandle) (ASTHandle, error) {
	return m.regexConstant(24, "re.allchar", "re.allchar", sort)
}
