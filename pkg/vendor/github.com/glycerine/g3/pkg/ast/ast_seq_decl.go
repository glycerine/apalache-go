package ast

// Source: src/ast/seq_decl_plugin.cpp:620-825.

func canonicalSequenceKind(kind int32) int32 {
	switch kind {
	case 29:
		return 7
	case 30:
		return ASTSequenceOpConcat
	case 31:
		return 9
	case 32:
		return 5
	case 33:
		return 3
	case 34:
		return 4
	case 35:
		return 12
	case 36:
		return 11
	case 37:
		return 8
	case 38:
		return 6
	case 39:
		return 10
	case 40:
		return 22
	case 41:
		return 24
	default:
		return kind
	}
}

func (m *ASTManager) sequenceDecl(kind int32, parameters []ASTParameter, domain []ASTHandle, rangeHandle ASTHandle) (astNodeRef, error) {
	p := m.sequencePlugin
	if p == nil {
		return astNodeRef{}, ErrASTDomain
	}
	if !p.initialized {
		if err := m.initializeSequence(p); err != nil {
			return astNodeRef{}, err
		}
	}
	if kind < 0 || kind >= int32(len(p.signatures)) {
		return astNodeRef{}, ErrASTDomain
	}
	domainRefs := make([]astNodeRef, len(domain))
	for i, handle := range domain {
		ref, err := m.requireKind(handle, ASTSort)
		if err != nil {
			return astNodeRef{}, err
		}
		domainRefs[i] = ref
	}
	var rangeSort astNodeRef
	if !rangeHandle.IsNull() {
		var err error
		rangeSort, err = m.requireKind(rangeHandle, ASTSort)
		if err != nil {
			return astNodeRef{}, err
		}
	}
	if kind == ASTSequenceOpStringConst {
		if len(parameters) != 1 || len(domain) != 0 {
			return astNodeRef{}, ErrASTDomain
		}
		parameter, err := m.parameter(parameters[0])
		if err != nil || parameter.Kind != ASTParameterSymbol {
			return astNodeRef{}, ErrASTDomain
		}
		return m.pluginDecl(p.names[0], p.family, kind, parameters, nil, p.stringSort, ASTFuncFlags{}, false)
	}
	if kind == 42 {
		if len(parameters) != 1 || rangeSort.slot == 0 {
			return astNodeRef{}, ErrASTDomain
		}
		parameter, err := m.parameter(parameters[0])
		if err != nil || parameter.Kind != ASTParameterSymbol {
			return astNodeRef{}, ErrASTDomain
		}
		return m.pluginDecl(parameter.symbol, p.family, kind, parameters, domain, rangeSort, ASTFuncFlags{}, false)
	}
	canonicalKind := canonicalSequenceKind(kind)
	signature := p.signatures[kind]
	if signature == nil {
		return astNodeRef{}, ErrASTDomain
	}
	if rangeSort.slot == 0 && (canonicalKind == 22 || canonicalKind == 23 || canonicalKind == 24) {
		rangeSort = p.regex
	}
	rightAssociative := canonicalKind == ASTSequenceOpConcat || canonicalKind == 17 || canonicalKind == 18 || canonicalKind == 19
	var result astNodeRef
	if canonicalKind == 20 && (len(domainRefs) == 2 || len(domainRefs) == 3) {
		integer := m.arithmeticPlugin.fields[1]
		if domainRefs[0] != p.regex || domainRefs[1] != integer || len(domainRefs) == 3 && domainRefs[2] != integer {
			return astNodeRef{}, ErrASTDomain
		}
		result = domainRefs[0]
	} else {
		matchDomain := domainRefs
		if canonicalKind == 10 && len(domainRefs) == 2 {
			matchDomain = append(append([]astNodeRef(nil), domainRefs...), m.arithmeticPlugin.fields[1])
		}
		var err error
		result, err = m.sequenceMatchSignature(signature, matchDomain, rangeSort, rightAssociative)
		if err != nil {
			return astNodeRef{}, err
		}
	}
	if kind == ASTSequenceOpEmpty {
		if len(parameters) != 0 || len(domain) != 0 {
			return astNodeRef{}, ErrASTDomain
		}
		if result == p.stringSort {
			empty, err := m.Symbols.Intern("")
			if err != nil {
				return astNodeRef{}, err
			}
			parameter, err := m.newParameterSymbol(empty)
			if err != nil {
				return astNodeRef{}, err
			}
			defer m.dropParameter(parameter)
			return m.pluginDecl(p.names[0], p.family, ASTSequenceOpStringConst, []ASTParameter{parameter}, nil, p.stringSort, ASTFuncFlags{}, false)
		}
		parameter, err := m.newParameterAST(m.external(result))
		if err != nil {
			return astNodeRef{}, err
		}
		defer m.dropParameter(parameter)
		return m.pluginDecl(signature.name, p.family, kind, []ASTParameter{parameter}, nil, result, ASTFuncFlags{}, false)
	}
	if canonicalKind == 20 {
		if len(domain) == 1 {
			if len(parameters) < 1 || len(parameters) > 2 {
				return astNodeRef{}, ErrASTDomain
			}
			for _, token := range parameters {
				parameter, e := m.parameter(token)
				if e != nil || parameter.Kind != ASTParameterInt {
					return astNodeRef{}, ErrASTDomain
				}
			}
		} else if len(domain) != 2 && len(domain) != 3 {
			return astNodeRef{}, ErrASTDomain
		}
	} else if len(parameters) != 0 {
		return astNodeRef{}, ErrASTDomain
	}
	name := signature.name
	stringInput := len(domainRefs) != 0 && domainRefs[0] == p.stringSort
	if stringInput && canonicalKind == kind {
		aliases := map[int32]int{ASTSequenceOpConcat: 30, 3: 33, 4: 34, 5: 32, 6: 38, 7: 29, 8: 37, 9: 31, 10: 39, 11: 36, 12: 35}
		if alias, ok := aliases[kind]; ok {
			name = p.signatures[alias].name
		}
	}
	if canonicalKind == 22 && result == p.regex {
		name = p.signatures[40].name
	}
	flags := ASTFuncFlags{}
	declarationDomain := domain
	if rightAssociative {
		flags.RightAssociative = true
		declarationDomain = []ASTHandle{m.external(result), m.external(result)}
	}
	return m.pluginDecl(name, p.family, canonicalKind, parameters, declarationDomain, result, flags, false)
}
