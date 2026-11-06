package base

// Source: src/muz/base/dl_rule.cpp rule_manager::to_formula.

import "github.com/glycerine/g3/pkg/ast"

// RuleFormula converts an owned rule to an owning universally quantified Horn
// formula after checking that it belongs to this manager.
func (m *RuleManager) RuleFormula(rule *Rule) (ast.ASTRef, error) {
	if err := m.open(); err != nil {
		return ast.ASTRef{}, err
	}
	state, err := rule.open()
	if err != nil || state.manager != m.manager {
		return ast.ASTRef{}, ErrRuleOwner
	}
	return rule.ToFormula()
}

// Formula is the Rule receiver spelling used by retained consumers.
func (r *Rule) Formula() (ast.ASTRef, error) {
	return r.ToFormula()
}
