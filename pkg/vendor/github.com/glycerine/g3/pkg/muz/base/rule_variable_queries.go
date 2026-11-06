package base

// Source: src/muz/base/dl_rule.cpp rule::get_used_vars and rule::get_vars.

import "github.com/glycerine/g3/pkg/ast"

// UsedVariables collects the sorts of every free de Bruijn variable in the
// head and all tails.
func (r *Rule) UsedVariables() (*ast.ASTUsedVars, error) {
	state, err := r.open()
	if err != nil {
		return nil, err
	}
	used, err := ast.NewASTUsedVars(state.manager)
	if err != nil {
		return nil, err
	}
	if err = used.Process(state.head); err != nil {
		return nil, err
	}
	for _, tail := range state.tail {
		if err = used.Process(tail.handle); err != nil {
			return nil, err
		}
	}
	return used, nil
}

// VariableSorts returns the dense variable-sort vector used by native rule
// consumers. Gaps are represented by Bool, matching rule::get_vars.
func (r *Rule) VariableSorts() ([]ast.ASTHandle, error) {
	state, err := r.open()
	if err != nil {
		return nil, err
	}
	used, err := r.UsedVariables()
	if err != nil {
		return nil, err
	}
	boolean, err := state.manager.MkBoolSort()
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, used.MaxFoundVarIndexPlusOne())
	for index := range result {
		result[index] = used.Contains(uint32(index))
		if result[index].IsNull() {
			result[index] = boolean
		}
	}
	return result, nil
}
