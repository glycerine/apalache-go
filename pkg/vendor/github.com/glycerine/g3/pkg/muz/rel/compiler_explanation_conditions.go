package rel

// Source: dl_compiler.cpp interpreted-tail free-variable collection and
// ascending variable-index binding before make_add_unbound_column.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

type explanationConditionVariable struct {
	index uint32
	sort  ast.ASTHandle
}

func explanationInterpretedVariables(rule *base.Rule) ([]explanationConditionVariable, error) {
	manager, err := rule.ASTManager()
	if err != nil {
		return nil, err
	}
	used, err := ast.NewASTUsedVars(manager)
	if err != nil {
		return nil, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, err
	}
	count, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	for index := uninterpreted; index < count; index++ {
		condition, negative, err := rule.Tail(index)
		if err != nil {
			return nil, err
		}
		if negative {
			return nil, ErrCompilerUnsupported
		}
		if err := used.Process(condition); err != nil {
			return nil, err
		}
	}
	var variables []explanationConditionVariable
	for index := uint32(0); index < used.MaxFoundVarIndexPlusOne(); index++ {
		if sort := used.Contains(index); !sort.IsNull() {
			variables = append(variables, explanationConditionVariable{index: index, sort: sort})
		}
	}
	return variables, nil
}

func planExplanationConditionColumns(relations *RelationManager, rule *base.Rule, head ast.ASTHandle, headFinite, headExplanations, bodyFinite int, plan *explanationRuleLayout, available map[uint32][]int) error {
	variables, err := explanationInterpretedVariables(rule)
	if err != nil {
		return err
	}
	var headProof uint32
	if headExplanations == 1 {
		argument, err := relations.manager.GetArg(head, uint32(headFinite))
		if err != nil {
			return err
		}
		kind, err := relations.manager.GetKind(argument)
		if err != nil {
			return err
		}
		if kind != ast.ASTVar {
			return ErrCompilerUnsupported
		}
		headProof, err = relations.manager.GetVarIndex(argument)
		if err != nil {
			return err
		}
	}
	for _, variable := range variables {
		if actual, found := plan.sorts[variable.index]; found {
			if actual != variable.sort {
				return ErrCompilerUnsupported
			}
			continue
		}
		ruleSort, err := relations.manager.IsDatalogRuleSort(variable.sort)
		if err != nil {
			return err
		}
		if ruleSort {
			// The source explanation transform assigns this fresh slot after
			// all finite columns exist. Other unbound proof slots are unsupported.
			if headExplanations == 1 && variable.index == headProof {
				continue
			}
			return ErrCompilerUnsupported
		}
		if _, err := relations.sortDomain(variable.sort); err != nil {
			if relations.explanationNumericKind == "" {
				return errors.Join(ErrCompilerUnsupported, err)
			}
			accepted, numericErr := relationExplanationNumericCanHandle(relations, relations.explanationNumericKind, []ast.ASTHandle{variable.sort})
			if numericErr != nil || !accepted {
				return errors.Join(ErrCompilerUnsupported, err, numericErr)
			}
		}
		column := bodyFinite + len(plan.appendFinite)
		plan.appendFinite = append(plan.appendFinite, explanationFiniteColumn{sort: variable.sort})
		plan.columns[variable.index], plan.sorts[variable.index] = column, variable.sort
		available[variable.index] = []int{column}
	}
	return nil
}
