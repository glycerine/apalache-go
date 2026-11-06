package base

// Source: src/muz/base/dl_rule.cpp rule_manager::display_smt2 retained string
// consumer convenience.

import "bytes"

func (m *RuleManager) RuleFormulaString(rule *Rule) (string, error) {
	var out bytes.Buffer
	if err := m.WriteRuleFormula(&out, rule); err != nil {
		return "", err
	}
	return out.String(), nil
}

func (r *Rule) FormulaString() (string, error) {
	var out bytes.Buffer
	if err := r.WriteFormula(&out); err != nil {
		return "", err
	}
	return out.String(), nil
}
