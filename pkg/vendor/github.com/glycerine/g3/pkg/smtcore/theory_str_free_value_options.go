package smtcore

// Source: src/smt/theory_str.cpp gen_free_var_options.

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) generateStringFreeValueOptions(variable, lengthIndicator ast.ASTHandle, lengthText string, callback ast.ASTHandle, callbackValue string) (ast.ASTRef, error) {
	parsed, err := strconv.ParseInt(lengthText, 10, 32)
	if err != nil || parsed < 0 || parsed == 1<<31-1 {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	length := int32(parsed)
	previous := t.testers.valueIndicators[variable][length]
	empty := true
	for _, entry := range previous {
		if _, active := t.internalVariables[entry.variable]; active {
			empty = false
			break
		}
	}
	create := func(tries int32) (ast.ASTHandle, error) {
		indicator, err := t.freshStringValueTester(variable, length, tries)
		if err != nil {
			return indicator, err
		}
		t.testers.valueSubjects[indicator] = variable
		if t.testers.valueIndicators[variable] == nil {
			t.testers.valueIndicators[variable] = make(map[int32][]smtStringScopedIndicator)
		}
		entry := smtStringScopedIndicator{level: t.level, variable: indicator}
		t.testers.valueIndicators[variable][length] = append(t.testers.valueIndicators[variable][length], entry)
		return indicator, nil
	}
	if empty {
		indicator, err := create(0)
		if err != nil {
			return ast.ASTRef{}, err
		}
		// Native code appends even when old entries exist but are inactive.
		return t.generateStringValueOptions(variable, lengthIndicator, indicator, lengthText, 0)
	}
	if int64(len(previous)) > 1<<31-1 {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	i := 0
	for ; i < len(previous); i++ {
		indicator := previous[i].variable
		if _, active := t.internalVariables[indicator]; !active {
			continue
		}
		if indicator == callback {
			break
		}
		_, assigned, err := t.eqcStringValue(indicator)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if assigned {
			continue
		}
		if err = t.refreshStringVariable(indicator); err != nil {
			return ast.ASTRef{}, err
		}
		root, err := t.generateStringValueOptions(variable, lengthIndicator, indicator, lengthText, int32(i))
		if err != nil {
			return ast.ASTRef{}, err
		}
		m := t.state.manager
		formula, err := m.RefGet(root)
		if err == nil {
			err = t.assertAxiom(formula)
		}
		err = errors.Join(err, m.DelRef(&root))
		if err != nil {
			return ast.ASTRef{}, err
		}
	}
	if callbackValue != "more" {
		return ast.ASTRef{}, nil
	}
	if i >= 1<<31-1 {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	var indicator ast.ASTHandle
	if i+1 < len(previous) {
		indicator = previous[i+1].variable
		if err = t.refreshStringVariable(indicator); err != nil {
			return ast.ASTRef{}, err
		}
	} else {
		indicator, err = create(int32(i + 1))
		if err != nil {
			return ast.ASTRef{}, err
		}
	}
	return t.generateStringValueOptions(variable, lengthIndicator, indicator, lengthText, int32(i+1))
}
