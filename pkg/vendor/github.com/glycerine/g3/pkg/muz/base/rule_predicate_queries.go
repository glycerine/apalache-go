package base

// Source: src/muz/base/dl_rule.cpp rule_manager::contains_predicate.

import "github.com/glycerine/g3/pkg/ast"

// ContainsPredicate reports whether expression contains an application whose
// declaration is registered as a Datalog predicate.
func (m *RuleManager) ContainsPredicate(expression ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	found := false
	err := walkRuleExpression(m.manager, expression, func(current ast.ASTHandle, kind ast.ASTKind) (bool, error) {
		if kind != ast.ASTApp {
			return true, nil
		}
		declaration, err := m.manager.GetDecl(current)
		if err != nil {
			return false, err
		}
		_, found = m.predicates[declaration]
		return !found, nil
	})
	return found, err
}
