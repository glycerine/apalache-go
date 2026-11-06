package base

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// ReplaceRuleHead is the source copy constructor with a specified head. The
// tail, signs, and positive/uninterpreted boundaries are copied verbatim.
func (m *RuleManager) ReplaceRuleHead(source *Rule, head ast.ASTHandle, name string) (*Rule, error) {
	label, err := m.ruleName(name)
	if err != nil {
		return nil, err
	}
	return m.ReplaceRuleHeadWithSymbol(source, head, label)
}

// ReplaceRuleHeadWithSymbol copies a rule with a specified head and label.
func (m *RuleManager) ReplaceRuleHeadWithSymbol(source *Rule, head ast.ASTHandle, name symbol.Symbol) (*Rule, error) {
	return m.cloneRuleState(source, head, name)
}

// ReplaceRuleHeadNamed preserves the source rule name.
func (m *RuleManager) ReplaceRuleHeadNamed(source *Rule, head ast.ASTHandle) (*Rule, error) {
	name, err := source.NameSymbol()
	if err != nil {
		return nil, err
	}
	return m.ReplaceRuleHeadWithSymbol(source, head, name)
}
