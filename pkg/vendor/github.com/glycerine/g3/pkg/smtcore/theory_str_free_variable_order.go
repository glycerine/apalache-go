package smtcore

// Source: src/smt/theory_str.cpp get_var_in_eqc and process_free_var's
// representative/leaf/standalone selection before tester generation.

import "github.com/glycerine/g3/pkg/ast"

func (t *SMTStringTheory) stringClassVariables(expression ast.ASTHandle) []ast.ASTHandle {
	var variables []ast.ASTHandle
	for _, member := range t.stringClass(expression) {
		if _, variable := t.variableSet[member]; variable {
			variables = append(variables, member)
		}
	}
	stringSortHandles(variables)
	return variables
}

// The two source emission loops call the same tester generator. Return their
// combined visitation order while keeping the actual selection: deduplicate
// registered variables in private classes, then leaf variables before alone
// variables with unknown length. Source unordered sets use Go handle order.
func (t *SMTStringTheory) stringFreeVariableOrder(candidates map[ast.ASTHandle]int) ([]ast.ASTHandle, error) {
	representatives := make(map[ast.ASTHandle]struct{})
	for _, candidate := range stringOrderedCounts(candidates) {
		if _, regex := t.regexVariables[candidate]; regex {
			continue
		}
		duplicate := false
		for _, member := range t.stringClassVariables(candidate) {
			if _, seen := representatives[member]; seen {
				duplicate = true
				break
			}
		}
		if !duplicate {
			representatives[candidate] = struct{}{}
		}
	}
	var ordered []ast.ASTHandle
	for representative := range representatives {
		ordered = append(ordered, representative)
	}
	stringSortHandles(ordered)
	var leaf, alone []ast.ASTHandle
	for _, variable := range ordered {
		// input_var_in_len has one source insertion, guarded by String sort
		// and is_length simultaneously. Actual length terms have Int sort,
		// so that cache is always empty for retained well-typed consumers.
		if t.state.context.enodes[variable] == nil {
			if err := t.state.Internalize(variable, false); err != nil {
				return nil, err
			}
		}
		node := t.state.context.enodes[variable]
		if node == nil {
			return nil, ast.ErrASTDomain
		}
		standalone := true
		for _, parent := range node.Parents() {
			concat, err := t.state.manager.IsSequenceConcat(parent.Owner())
			if err != nil {
				return nil, err
			}
			if concat {
				standalone = false
				break
			}
		}
		if standalone {
			_, known, err := t.lengthValue(variable)
			if err != nil {
				return nil, err
			}
			if !known {
				alone = append(alone, variable)
				continue
			}
		}
		leaf = append(leaf, variable)
	}
	return append(leaf, alone...), nil
}
