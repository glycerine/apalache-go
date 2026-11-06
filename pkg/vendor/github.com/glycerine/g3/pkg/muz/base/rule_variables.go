package base

// Source: src/muz/base/dl_rule.cpp rule_manager variable collection helpers
// retained by filter and negated-tail transformations.

import "github.com/glycerine/g3/pkg/ast"

func (m *RuleManager) IsRegisteredPredicate(declaration ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	_, found := m.predicates[declaration]
	return found, nil
}

func collectExpressionVariables(manager *ast.ASTManager, expressions []ast.ASTHandle) (map[uint32]ast.ASTHandle, error) {
	used, err := ast.NewASTUsedVars(manager)
	if err != nil {
		return nil, err
	}
	used.Reset()
	for _, expression := range expressions {
		if err = used.Process(expression); err != nil {
			return nil, err
		}
	}
	result := make(map[uint32]ast.ASTHandle, used.NumVars())
	for index := uint32(0); index < used.MaxFoundVarIndexPlusOne(); index++ {
		if sort := used.Contains(index); !sort.IsNull() {
			result[index] = sort
		}
	}
	return result, nil
}

// VariablesOutsideTail returns free variables occurring in the rule outside
// the selected tail position.
func (m *RuleManager) VariablesOutsideTail(rule *Rule, excluded int) (map[uint32]ast.ASTHandle, error) {
	if err := m.checkRuleOwner(rule); err != nil {
		return nil, err
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	expressions := []ast.ASTHandle{head}
	total, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	for index := 0; index < total; index++ {
		if index == excluded {
			continue
		}
		tail, _, getErr := rule.Tail(index)
		if getErr != nil {
			return nil, getErr
		}
		expressions = append(expressions, tail)
	}
	return collectExpressionVariables(m.manager, expressions)
}

// TailVariables returns free variables occurring anywhere in the rule tail.
func (m *RuleManager) TailVariables(rule *Rule) (map[uint32]ast.ASTHandle, error) {
	if err := m.checkRuleOwner(rule); err != nil {
		return nil, err
	}
	total, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	expressions := make([]ast.ASTHandle, 0, total)
	for index := 0; index < total; index++ {
		tail, _, getErr := rule.Tail(index)
		if getErr != nil {
			return nil, getErr
		}
		expressions = append(expressions, tail)
	}
	return collectExpressionVariables(m.manager, expressions)
}

// ExpressionVariables returns the free variable indices and their sorts.
func (m *RuleManager) ExpressionVariables(expression ast.ASTHandle) (map[uint32]ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	return collectExpressionVariables(m.manager, []ast.ASTHandle{expression})
}
