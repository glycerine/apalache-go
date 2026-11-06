package base

// Source: src/muz/base/dl_rule.cpp rule_manager::mk(rule const*, app*, symbol).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func (m *RuleManager) cloneRuleState(source *Rule, head ast.ASTHandle, name symbol.Symbol) (_ *Rule, resultErr error) {
	state, err := m.checkRuleAnalysisOwner(source)
	if err != nil {
		return nil, err
	}
	if err := m.checkRuleName(name); err != nil {
		return nil, err
	}
	if err = m.validHead(head); err != nil {
		return nil, err
	}
	references := make([]ast.ASTRef, 0, len(state.tail)+1)
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, closeASTRefs(m.manager, references))
		}
	}()
	headReference, err := m.manager.NewRef(head)
	if err != nil {
		return nil, err
	}
	references = append(references, headReference)
	tail := make([]ruleTail, len(state.tail))
	for index, sourceTail := range state.tail {
		reference, getErr := m.manager.NewRef(sourceTail.handle)
		if getErr != nil {
			return nil, getErr
		}
		references = append(references, reference)
		tail[index] = ruleTail{handle: sourceTail.handle, reference: reference, negated: sourceTail.negated}
	}
	return &Rule{state: &ruleState{
		manager:       m.manager,
		head:          head,
		headReference: headReference,
		tail:          tail,
		positive:      state.positive,
		uninterpreted: state.uninterpreted,
		name:          name,
		references:    1,
		accounting:    NewAccountedObject(),
	}}, nil
}
