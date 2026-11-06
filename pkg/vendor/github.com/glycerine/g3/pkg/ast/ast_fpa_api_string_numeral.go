package ast

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

// MkFPANumeralString parses the source API numeral spelling at the target
// floating-point sort's precision using round-toward-zero conversion.
func (m *ASTManager) MkFPANumeralString(value string, sort ASTHandle) (out ASTHandle, result error) {
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
	if err = plugin.numeric.SetFormattedLiteral(&frame.value, ebits, sbits, unumeral.ArbitraryFloatTowardZero, value); err != nil {
		return out, err
	}
	node, result = plugin.mkNumeral(&frame.value, nil)
	return out, result
}
