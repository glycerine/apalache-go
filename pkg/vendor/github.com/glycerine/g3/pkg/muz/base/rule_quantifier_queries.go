package base

// Source: src/muz/base/dl_rule.cpp rule_manager::has_quantifiers overloads.

import "github.com/glycerine/g3/pkg/ast"

// RuleQuantifiers records which source quantifier forms occur in interpreted
// tails. Rules cannot contain quantifiers in their predicate tail.
type RuleQuantifiers struct {
	Existential bool
	Universal   bool
}

func (m *RuleManager) Quantifiers(rule *Rule) (RuleQuantifiers, error) {
	state, err := m.checkRuleAnalysisOwner(rule)
	if err != nil {
		return RuleQuantifiers{}, err
	}
	result := RuleQuantifiers{}
	for index := state.uninterpreted; index < len(state.tail); index++ {
		err = walkRuleExpression(m.manager, state.tail[index].handle, func(current ast.ASTHandle, kind ast.ASTKind) (bool, error) {
			if kind != ast.ASTQuantifier {
				return true, nil
			}
			forall, err := m.manager.IsForall(current)
			if err != nil {
				return false, err
			}
			if forall {
				result.Universal = true
			} else {
				result.Existential = true
			}
			return true, nil
		})
		if err != nil {
			return RuleQuantifiers{}, err
		}
	}
	return result, nil
}

func (m *RuleManager) HasRuleQuantifiers(rule *Rule) (bool, error) {
	quantifiers, err := m.Quantifiers(rule)
	return quantifiers.Existential || quantifiers.Universal, err
}
