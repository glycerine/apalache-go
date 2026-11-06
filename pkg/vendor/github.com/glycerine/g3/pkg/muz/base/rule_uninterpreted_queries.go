package base

// Source: src/muz/base/dl_rule.h uninterpreted_function_finder_proc and
// src/muz/base/dl_rule.cpp rule_manager::has_uninterpreted_non_predicates.

import "github.com/glycerine/g3/pkg/ast"

// UninterpretedNonPredicate returns the first non-predicate uninterpreted
// declaration in an interpreted tail. Multi-constructor datatype accessors
// are classified the same way as the native finder.
func (m *RuleManager) UninterpretedNonPredicate(rule *Rule) (ast.ASTHandle, bool, error) {
	state, err := m.checkRuleAnalysisOwner(rule)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	var declaration ast.ASTHandle
	found := false
	for index := state.uninterpreted; index < len(state.tail) && !found; index++ {
		err = walkRuleExpression(m.manager, state.tail[index].handle, func(current ast.ASTHandle, kind ast.ASTKind) (bool, error) {
			if kind != ast.ASTApp {
				return true, nil
			}
			candidate, err := m.manager.GetDecl(current)
			if err != nil {
				return false, err
			}
			family, err := m.manager.GetFamilyID(candidate)
			if err != nil {
				return false, err
			}
			if family == ast.NullFamilyID {
				rangeSort, err := m.manager.GetRange(candidate)
				if err != nil {
					return false, err
				}
				ruleSort, err := m.manager.IsDatalogRuleSort(rangeSort)
				if err != nil {
					return false, err
				}
				if !ruleSort {
					declaration, found = candidate, true
					return false, nil
				}
			}
			_, accessor, err := m.manager.DatatypeAccessorConstructor(candidate)
			if err != nil || !accessor {
				return true, err
			}
			argument, err := m.manager.GetArg(current, 0)
			if err != nil {
				return false, err
			}
			sort, err := m.manager.GetSort(argument)
			if err != nil {
				return false, err
			}
			constructors, err := m.manager.DatatypeConstructors(sort)
			if err != nil {
				return false, err
			}
			if len(constructors) > 1 {
				declaration, found = candidate, true
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
	}
	return declaration, found, nil
}

func (m *RuleManager) HasUninterpretedNonPredicates(rule *Rule) (bool, error) {
	_, found, err := m.UninterpretedNonPredicate(rule)
	return found, err
}
