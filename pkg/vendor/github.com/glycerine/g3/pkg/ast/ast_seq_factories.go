package ast

// Source: src/ast/seq_decl_plugin.cpp:620-672,755-767;
// src/ast/seq_decl_plugin.h:234-244.

// MkSequenceEmpty constructs the canonical empty value for a sequence sort.
// String sorts use the source empty-string specialization.
func (m *ASTManager) MkSequenceEmpty(sort ASTHandle) (ASTHandle, error) {
	if err := m.prepareSequencePlugin(); err != nil {
		return ASTHandle{}, err
	}
	if err := m.lock(); err != nil {
		return ASTHandle{}, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(sort, ASTSort)
	if err != nil {
		return ASTHandle{}, err
	}
	value, err := m.mkSequenceEmpty(r)
	return m.external(value), err
}

// MkSequenceUnit constructs seq.unit with its result sort inferred from value.
func (m *ASTManager) MkSequenceUnit(value ASTHandle) (ASTHandle, error) {
	if err := m.prepareSequencePlugin(); err != nil {
		return ASTHandle{}, err
	}
	if err := m.lock(); err != nil {
		return ASTHandle{}, err
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(value)
	if err != nil {
		return ASTHandle{}, err
	}
	elementSort, err := m.sortOf(r)
	if err != nil {
		return ASTHandle{}, err
	}
	parameter := astParameterRecord{Kind: ASTParameterAST, ast: elementSort}
	sequenceSort, err := m.sequenceSort(m.sequencePlugin, 0, &parameter)
	if err != nil {
		return ASTHandle{}, err
	}
	decl, err := m.makePluginDeclaration(m.sequencePlugin.family, "seq.unit", ASTSequenceOpUnit, []astNodeRef{elementSort}, sequenceSort, ASTFuncFlags{})
	if err != nil {
		return ASTHandle{}, err
	}
	app, err := m.mkApp(decl, []ASTHandle{value})
	return m.external(app), err
}

// MkSequenceConcat constructs the source binary associative concatenation.
func (m *ASTManager) MkSequenceConcat(left, right ASTHandle) (ASTHandle, error) {
	if err := m.prepareSequencePlugin(); err != nil {
		return ASTHandle{}, err
	}
	if err := m.lock(); err != nil {
		return ASTHandle{}, err
	}
	defer m.mu.Unlock()
	l, err := m.requireExpr(left)
	if err != nil {
		return ASTHandle{}, err
	}
	r, err := m.requireExpr(right)
	if err != nil {
		return ASTHandle{}, err
	}
	ls, err := m.sortOf(l)
	if err != nil {
		return ASTHandle{}, err
	}
	rs, err := m.sortOf(r)
	if err != nil || ls != rs {
		return ASTHandle{}, ErrASTDomain
	}
	family, kind, err := m.declIdentity(ls)
	if err != nil || family != m.sequencePlugin.family || kind != 0 {
		return ASTHandle{}, ErrASTDomain
	}
	name := "seq.++"
	if ls == m.sequencePlugin.stringSort {
		name = "str.++"
	}
	decl, err := m.makePluginDeclaration(m.sequencePlugin.family, name, ASTSequenceOpConcat, []astNodeRef{ls, ls}, ls, ASTFuncFlags{RightAssociative: true})
	if err != nil {
		return ASTHandle{}, err
	}
	app, err := m.mkApp(decl, []ASTHandle{left, right})
	return m.external(app), err
}

func (m *ASTManager) sequenceApplication(kind int32, genericName, stringName string, arguments []ASTHandle, result astNodeRef) (ASTHandle, error) {
	refs := make([]astNodeRef, len(arguments))
	var sequenceSort astNodeRef
	for i, argument := range arguments {
		ref, err := m.requireExpr(argument)
		if err != nil {
			return ASTHandle{}, err
		}
		refs[i], err = m.sortOf(ref)
		if err != nil {
			return ASTHandle{}, err
		}
		if i == 0 {
			sequenceSort = refs[i]
		}
	}
	family, sortKind, err := m.declIdentity(sequenceSort)
	if err != nil || family != m.sequencePlugin.family || sortKind != 0 {
		return ASTHandle{}, ErrASTDomain
	}
	name := genericName
	if sequenceSort == m.sequencePlugin.stringSort {
		name = stringName
	}
	decl, err := m.makePluginDeclaration(m.sequencePlugin.family, name, kind, refs, result, ASTFuncFlags{})
	if err != nil {
		return ASTHandle{}, err
	}
	app, err := m.mkApp(decl, arguments)
	return m.external(app), err
}

func (m *ASTManager) withSequenceFactory(body func() (ASTHandle, error)) (ASTHandle, error) {
	if err := m.prepareSequencePlugin(); err != nil {
		return ASTHandle{}, err
	}
	if err := m.lock(); err != nil {
		return ASTHandle{}, err
	}
	defer m.mu.Unlock()
	return body()
}

func (m *ASTManager) MkSequenceLength(sequence ASTHandle) (ASTHandle, error) {
	return m.withSequenceFactory(func() (ASTHandle, error) {
		return m.sequenceApplication(9, "seq.len", "str.len", []ASTHandle{sequence}, m.arithmeticPlugin.fields[1])
	})
}

func (m *ASTManager) sequenceRelation(kind int32, genericName, stringName string, left, right ASTHandle) (ASTHandle, error) {
	return m.withSequenceFactory(func() (ASTHandle, error) {
		leftRef, err := m.requireExpr(left)
		if err != nil {
			return ASTHandle{}, err
		}
		ls, err := m.sortOf(leftRef)
		if err != nil {
			return ASTHandle{}, err
		}
		rightRef, err := m.requireExpr(right)
		if err != nil {
			return ASTHandle{}, err
		}
		rs, err := m.sortOf(rightRef)
		if err != nil || ls != rs {
			return ASTHandle{}, ErrASTDomain
		}
		return m.sequenceApplication(kind, genericName, stringName, []ASTHandle{left, right}, m.boolSort)
	})
}

func (m *ASTManager) MkSequencePrefix(left, right ASTHandle) (ASTHandle, error) {
	return m.sequenceRelation(3, "seq.prefixof", "str.prefixof", left, right)
}

func (m *ASTManager) MkSequenceSuffix(left, right ASTHandle) (ASTHandle, error) {
	return m.sequenceRelation(4, "seq.suffixof", "str.suffixof", left, right)
}

func (m *ASTManager) MkSequenceContains(left, right ASTHandle) (ASTHandle, error) {
	return m.sequenceRelation(5, "seq.contains", "str.contains", left, right)
}

func (m *ASTManager) MkSequenceReplace(sequence, source, replacement ASTHandle) (ASTHandle, error) {
	return m.withSequenceFactory(func() (ASTHandle, error) {
		arguments := []ASTHandle{sequence, source, replacement}
		var sequenceSort astNodeRef
		for i, argument := range arguments {
			ref, err := m.requireExpr(argument)
			if err != nil {
				return ASTHandle{}, err
			}
			sort, err := m.sortOf(ref)
			if err != nil {
				return ASTHandle{}, err
			}
			if i == 0 {
				sequenceSort = sort
			} else if sort != sequenceSort {
				return ASTHandle{}, ErrASTDomain
			}
		}
		return m.sequenceApplication(7, "seq.replace", "str.replace", arguments, sequenceSort)
	})
}

func (m *ASTManager) MkIntToString(integer ASTHandle) (ASTHandle, error) {
	return m.withSequenceFactory(func() (ASTHandle, error) {
		ref, err := m.requireExpr(integer)
		if err != nil {
			return ASTHandle{}, err
		}
		sort, err := m.sortOf(ref)
		if err != nil || sort != m.arithmeticPlugin.fields[1] {
			return ASTHandle{}, ErrASTDomain
		}
		decl, err := m.makePluginDeclaration(m.sequencePlugin.family, "int.to.str", 27, []astNodeRef{sort}, m.sequencePlugin.stringSort, ASTFuncFlags{})
		if err != nil {
			return ASTHandle{}, err
		}
		app, err := m.mkApp(decl, []ASTHandle{integer})
		return m.external(app), err
	})
}

func (m *ASTManager) MkStringToInt(str ASTHandle) (ASTHandle, error) {
	return m.withSequenceFactory(func() (ASTHandle, error) {
		ref, err := m.requireExpr(str)
		if err != nil {
			return ASTHandle{}, err
		}
		sort, err := m.sortOf(ref)
		if err != nil || sort != m.sequencePlugin.stringSort {
			return ASTHandle{}, ErrASTDomain
		}
		decl, err := m.makePluginDeclaration(m.sequencePlugin.family, "str.to.int", 28, []astNodeRef{sort}, m.arithmeticPlugin.fields[1], ASTFuncFlags{})
		if err != nil {
			return ASTHandle{}, err
		}
		app, err := m.mkApp(decl, []ASTHandle{str})
		return m.external(app), err
	})
}

func (m *ASTManager) sequenceIndexed(kind int32, genericName, stringName string, sequence ASTHandle, indices ...ASTHandle) (ASTHandle, error) {
	return m.withSequenceFactory(func() (ASTHandle, error) {
		sequenceRef, err := m.requireExpr(sequence)
		if err != nil {
			return ASTHandle{}, err
		}
		sequenceSort, err := m.sortOf(sequenceRef)
		if err != nil {
			return ASTHandle{}, err
		}
		arguments := append([]ASTHandle{sequence}, indices...)
		for _, index := range indices {
			ref, e := m.requireExpr(index)
			if e != nil {
				return ASTHandle{}, e
			}
			sort, e := m.sortOf(ref)
			if e != nil || sort != m.arithmeticPlugin.fields[1] {
				return ASTHandle{}, ErrASTDomain
			}
		}
		return m.sequenceApplication(kind, genericName, stringName, arguments, sequenceSort)
	})
}

func (m *ASTManager) MkSequenceExtract(sequence, offset, length ASTHandle) (ASTHandle, error) {
	return m.sequenceIndexed(6, "seq.extract", "str.substr", sequence, offset, length)
}

func (m *ASTManager) MkSequenceAt(sequence, index ASTHandle) (ASTHandle, error) {
	return m.sequenceIndexed(8, "seq.at", "str.at", sequence, index)
}

func (m *ASTManager) MkSequenceIndex(sequence, subsequence, offset ASTHandle) (ASTHandle, error) {
	return m.withSequenceFactory(func() (ASTHandle, error) {
		sequenceRef, err := m.requireExpr(sequence)
		if err != nil {
			return ASTHandle{}, err
		}
		sequenceSort, err := m.sortOf(sequenceRef)
		if err != nil {
			return ASTHandle{}, err
		}
		subsequenceRef, err := m.requireExpr(subsequence)
		if err != nil {
			return ASTHandle{}, err
		}
		subsequenceSort, err := m.sortOf(subsequenceRef)
		if err != nil || sequenceSort != subsequenceSort {
			return ASTHandle{}, ErrASTDomain
		}
		offsetRef, err := m.requireExpr(offset)
		if err != nil {
			return ASTHandle{}, err
		}
		offsetSort, err := m.sortOf(offsetRef)
		if err != nil || offsetSort != m.arithmeticPlugin.fields[1] {
			return ASTHandle{}, ErrASTDomain
		}
		return m.sequenceApplication(10, "seq.indexof", "str.indexof", []ASTHandle{sequence, subsequence, offset}, m.arithmeticPlugin.fields[1])
	})
}
