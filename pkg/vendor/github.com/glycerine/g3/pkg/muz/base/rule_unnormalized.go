package base

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// ASTManager returns the borrowed AST manager used by this rule manager.
func (m *RuleManager) ASTManager() (*ast.ASTManager, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	return m.manager, nil
}

// NewUnnormalizedRule implements rule_manager::mk with normalize=false for
// transforms that preserve the existing variable indices after removing tails.
func (m *RuleManager) NewUnnormalizedRule(head ast.ASTHandle, tail []ast.ASTHandle, negated []bool, name string) (*Rule, error) {
	label, err := m.ruleName(name)
	if err != nil {
		return nil, err
	}
	return m.NewUnnormalizedRuleWithSymbol(head, tail, negated, label)
}

// NewUnnormalizedRuleWithSymbol keeps the supplied source label and indices.
func (m *RuleManager) NewUnnormalizedRuleWithSymbol(head ast.ASTHandle, tail []ast.ASTHandle, negated []bool, name symbol.Symbol) (*Rule, error) {
	return m.newRule(head, tail, negated, name, false)
}
