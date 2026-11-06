package ast

// Source: src/api/api_fpa.cpp:341-379.

// MkFPANumeralComponents constructs a numeral from the raw MPF sign,
// unbiased exponent and trailing significand tuple.
func (m *ASTManager) MkFPANumeralComponents(sign bool, exponent int64, significand uint64, sort ASTHandle) (out ASTHandle, result error) {
	if result = m.lock(); result != nil {
		return out, result
	}
	defer m.mu.Unlock()
	sortRef, err := m.requireKind(sort, ASTSort)
	if err != nil {
		return out, err
	}
	ebits, sbits, matched, err := m.floatingPointSortShape(sortRef)
	if err != nil {
		return out, err
	}
	if !matched {
		return out, ErrASTDomain
	}
	plugin := m.floatingPointPlugin
	if plugin == nil || plugin.sourceEnded {
		return out, ErrASTHandle
	}
	frame, err := plugin.beginNumeralProbe()
	if err != nil {
		return out, err
	}
	var node astNodeRef
	defer func() {
		plugin.finishNumeralConstruction(frame, &result, &node)
		if result == nil {
			out = m.external(node)
		}
	}()
	if err = plugin.numeric.SetComponentsUint64(&frame.value, ebits, sbits, sign, exponent, significand); err != nil {
		return out, err
	}
	node, result = plugin.mkNumeral(&frame.value, nil)
	return out, result
}
