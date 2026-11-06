package base

import "github.com/glycerine/g3/pkg/ast"

// ValidateHead exposes rule_manager::check_valid_head to retained consumers
// that construct direct-copy rules before committing them to a RuleSet.
func (m *RuleManager) ValidateHead(head ast.ASTHandle) error {
	if err := m.open(); err != nil {
		return err
	}
	return m.validHead(head)
}

// ValidateRule mirrors check_valid_rule: the native implementation delegates
// to head validation; tail semantics are established by construction.
func (m *RuleManager) ValidateRule(head ast.ASTHandle, _ []ast.ASTHandle) error {
	return m.ValidateHead(head)
}
