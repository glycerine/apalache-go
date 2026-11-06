package base

import "github.com/glycerine/g3/pkg/util/symbol"

// CopyRule directly copies source's native tail layout and assigns name.
func (m *RuleManager) CopyRule(source *Rule, name string) (*Rule, error) {
	label, err := m.ruleName(name)
	if err != nil {
		return nil, err
	}
	return m.CopyRuleWithSymbol(source, label)
}

// CopyRuleWithSymbol assigns an exact source symbol while copying the rule.
func (m *RuleManager) CopyRuleWithSymbol(source *Rule, name symbol.Symbol) (*Rule, error) {
	head, err := source.Head()
	if err != nil {
		return nil, err
	}
	return m.cloneRuleState(source, head, name)
}

// CloneRule is the name-preserving convenience form of CopyRule.
func (m *RuleManager) CloneRule(source *Rule) (*Rule, error) {
	name, err := source.NameSymbol()
	if err != nil {
		return nil, err
	}
	return m.CopyRuleWithSymbol(source, name)
}
