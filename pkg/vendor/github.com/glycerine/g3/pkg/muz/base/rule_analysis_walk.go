package base

// Source: src/muz/base/dl_rule.cpp contains_predicate_proc,
// uninterpreted_function_finder_proc, and quantifier_finder_proc traversal.

import "github.com/glycerine/g3/pkg/ast"

// walkRuleExpression visits each expression node once. Function declarations
// and sorts are intentionally not expression children in the source walker.
func walkRuleExpression(manager *ast.ASTManager, root ast.ASTHandle, visit func(ast.ASTHandle, ast.ASTKind) (bool, error)) error {
	seen := make(map[ast.ASTHandle]struct{})
	stack := []ast.ASTHandle{root}
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if _, found := seen[current]; found {
			continue
		}
		seen[current] = struct{}{}
		kind, err := manager.GetKind(current)
		if err != nil {
			return err
		}
		keepGoing, err := visit(current, kind)
		if err != nil {
			return err
		}
		if !keepGoing {
			return nil
		}
		switch kind {
		case ast.ASTApp:
			count, err := manager.GetNumArgs(current)
			if err != nil {
				return err
			}
			for index := count; index != 0; index-- {
				argument, err := manager.GetArg(current, index-1)
				if err != nil {
					return err
				}
				stack = append(stack, argument)
			}
		case ast.ASTQuantifier:
			body, err := manager.GetQuantifierBody(current)
			if err != nil {
				return err
			}
			stack = append(stack, body)
			patterns, err := manager.GetNumPatterns(current)
			if err != nil {
				return err
			}
			for index := patterns; index != 0; index-- {
				pattern, err := manager.GetPattern(current, index-1)
				if err != nil {
					return err
				}
				stack = append(stack, pattern)
			}
			noPatterns, err := manager.GetNumNoPatterns(current)
			if err != nil {
				return err
			}
			for index := noPatterns; index != 0; index-- {
				pattern, err := manager.GetNoPattern(current, index-1)
				if err != nil {
					return err
				}
				stack = append(stack, pattern)
			}
		}
	}
	return nil
}

func (m *RuleManager) checkRuleAnalysisOwner(rule *Rule) (*ruleState, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	state, err := rule.open()
	if err != nil {
		return nil, err
	}
	if state.manager != m.manager {
		return nil, ErrRuleOwner
	}
	return state, nil
}
