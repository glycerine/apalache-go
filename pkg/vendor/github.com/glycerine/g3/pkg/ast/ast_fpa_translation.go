package ast

// Source: src/ast/fpa_decl_plugin.cpp:176-180.

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// TranslateFloatingPointExternalParameter translates an attached FPA numeral
// parameter into the target plugin's value table.
func (m *ASTManager) TranslateFloatingPointExternalParameter(source ASTParameter, target *ASTManager) (result ASTParameter, err error) {
	if target == nil || target == m {
		return ASTParameter{}, ErrASTHandle
	}
	if err = m.lock(); err != nil {
		return ASTParameter{}, err
	}
	defer m.mu.Unlock()
	if err = target.lock(); err != nil {
		return ASTParameter{}, err
	}
	defer target.mu.Unlock()

	sourceRecord, err := m.parameterOfKind(source, ASTParameterExternal)
	if err != nil {
		return ASTParameter{}, err
	}
	if m.floatingPointPlugin == nil || target.floatingPointPlugin == nil {
		return ASTParameter{}, ErrASTDomain
	}
	value, err := m.floatingPointPlugin.values.at(sourceRecord.external)
	if err != nil {
		return ASTParameter{}, err
	}
	var translated unumeral.ArbitraryFloat
	if err = target.floatingPointPlugin.numeric.CopyFromOwner(&translated, m.floatingPointPlugin.numeric, &value); err != nil {
		return ASTParameter{}, err
	}
	id, err := target.floatingPointPlugin.mkValueID(&translated)
	cleanup := target.floatingPointPlugin.numeric.Del(&translated)
	if err != nil || cleanup != nil {
		if err == nil {
			cleanup = errors.Join(cleanup, target.floatingPointPlugin.recycledValueID(id))
		}
		return ASTParameter{}, errors.Join(err, cleanup)
	}
	result, err = target.allocParameter()
	if err != nil {
		_ = target.floatingPointPlugin.recycledValueID(id)
		return ASTParameter{}, err
	}
	r := target.parameterAt(result.slot)
	r.Kind, r.external = ASTParameterExternal, id
	return result, nil
}
