package ast

// Source: src/ast/seq_decl_plugin.cpp:358-498.

func (m *ASTManager) sequenceSortParameterIndex(sort astNodeRef) (uint32, bool, error) {
	if m.nodeAt(sort.slot).kind != ASTSort {
		return 0, false, ErrASTDomain
	}
	name := m.sortAt(sort.slot).name
	if !name.IsNumerical() {
		return 0, false, nil
	}
	index, err := name.Number()
	return index, err == nil, err
}

func (m *ASTManager) sequenceMatchSort(binding *[]astNodeRef, actual, pattern astNodeRef) (bool, error) {
	if actual == pattern {
		return true, nil
	}
	index, parameter, err := m.sequenceSortParameterIndex(pattern)
	if err != nil {
		return false, err
	}
	if parameter {
		for uint32(len(*binding)) <= index {
			*binding = append(*binding, astNodeRef{})
		}
		if (*binding)[index].slot != 0 && (*binding)[index] != actual {
			return false, nil
		}
		(*binding)[index] = actual
		return true, nil
	}
	actualFamily, actualKind, err := m.declIdentity(actual)
	if err != nil {
		return false, err
	}
	patternFamily, patternKind, err := m.declIdentity(pattern)
	if err != nil || actualFamily != patternFamily || actualKind != patternKind {
		return false, err
	}
	actualInfo, err := m.sortInfo(m.sortAt(actual.slot).info)
	if err != nil {
		return false, err
	}
	patternInfo, err := m.sortInfo(m.sortAt(pattern.slot).info)
	if err != nil || actualInfo.count != patternInfo.count {
		return false, err
	}
	for i := uint32(0); i < patternInfo.count; i++ {
		actualToken, e := m.infoParameter(actualInfo, i)
		if e != nil {
			return false, e
		}
		patternToken, e := m.infoParameter(patternInfo, i)
		if e != nil {
			return false, e
		}
		actualParameter, e := m.parameter(actualToken)
		if e != nil {
			return false, e
		}
		patternParameter, e := m.parameter(patternToken)
		if e != nil {
			return false, e
		}
		if actualParameter.Kind == ASTParameterAST && patternParameter.Kind == ASTParameterAST && m.nodeAt(actualParameter.ast.slot).kind == ASTSort && m.nodeAt(patternParameter.ast.slot).kind == ASTSort {
			matched, e := m.sequenceMatchSort(binding, actualParameter.ast, patternParameter.ast)
			if e != nil || !matched {
				return matched, e
			}
		}
	}
	return true, nil
}

func (m *ASTManager) sequenceApplyBinding(binding []astNodeRef, sort astNodeRef) (astNodeRef, error) {
	index, parameter, err := m.sequenceSortParameterIndex(sort)
	if err != nil {
		return astNodeRef{}, err
	}
	if parameter {
		if uint32(len(binding)) <= index || binding[index].slot == 0 {
			return astNodeRef{}, ErrASTDomain
		}
		return binding[index], nil
	}
	family, kind, err := m.declIdentity(sort)
	if err != nil || family != m.sequencePlugin.family || kind < 0 || kind > 1 {
		return sort, err
	}
	parameterSort, matched, err := m.sequenceSortParameter(sort, kind)
	if err != nil || !matched {
		return astNodeRef{}, ErrASTDomain
	}
	applied, err := m.sequenceApplyBinding(binding, parameterSort)
	if err != nil {
		return astNodeRef{}, err
	}
	p := astParameterRecord{Kind: ASTParameterAST, ast: applied}
	return m.sequenceSort(m.sequencePlugin, kind, &p)
}

func (m *ASTManager) sequenceMatchSignature(signature *astSequenceSignature, domain []astNodeRef, rangeSort astNodeRef, rightAssociative bool) (astNodeRef, error) {
	if signature == nil || signature.owner != m.sequencePlugin {
		return astNodeRef{}, ErrASTHandle
	}
	if rightAssociative {
		if len(domain) == 0 || len(signature.domain) == 0 {
			return astNodeRef{}, ErrASTDomain
		}
	} else if len(domain) != len(signature.domain) {
		return astNodeRef{}, ErrASTDomain
	}
	binding := make([]astNodeRef, 0, 1)
	for i, actual := range domain {
		var pattern astNodeRef
		if rightAssociative {
			pattern = signature.domain[0]
		} else {
			pattern = signature.domain[i]
		}
		matched, err := m.sequenceMatchSort(&binding, actual, pattern)
		if err != nil || !matched {
			if err == nil {
				err = ErrASTDomain
			}
			return astNodeRef{}, err
		}
	}
	if rangeSort.slot != 0 {
		matched, err := m.sequenceMatchSort(&binding, rangeSort, signature.rangeSort)
		if err != nil || !matched {
			if err == nil {
				err = ErrASTDomain
			}
			return astNodeRef{}, err
		}
	} else if len(domain) == 0 {
		return astNodeRef{}, ErrASTDomain
	}
	return m.sequenceApplyBinding(binding, signature.rangeSort)
}
