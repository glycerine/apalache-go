package smtcore

// Source: src/smt/theory_str.cpp more_len_tests, more_value_tests,
// free_var_attempt and the opening of handle_equality.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

// Consume the temporary expr_ref returned by a tester generator after the
// real assertion path has acquired its own lifetime root.
func (t *SMTStringTheory) assertStringTesterOptions(root ast.ASTRef) (result error) {
	if root == (ast.ASTRef{}) {
		return nil
	}
	m := t.state.manager
	defer func() { result = errors.Join(result, m.DelRef(&root)) }()
	formula, err := m.RefGet(root)
	if err != nil {
		return err
	}
	return t.assertAxiom(formula)
}

func (t *SMTStringTheory) moreStringLengthTests(tester ast.ASTHandle, value string) error {
	word, found := t.testers.lengthSubjects[tester]
	if !found {
		return nil
	}
	root, err := t.generateStringLengthValueOptions(word, tester, value)
	if err != nil {
		return err
	}
	return t.assertStringTesterOptions(root)
}

func (t *SMTStringTheory) moreStringValueTests(tester ast.ASTHandle, value string) error {
	word, found := t.testers.valueSubjects[tester]
	if !found {
		return ast.ErrASTDomain
	} // Native operator[] requires a registered producer.
	binary := false
	if t.parameterOwner != nil {
		options, err := t.stringParameterState()
		if err != nil {
			return err
		}
		binary = options.UseBinarySearch
	}
	var lengthTester ast.ASTHandle
	lengthText := ""
	if binary {
		stack := t.testers.binaryStacks[word]
		if len(stack) == 0 {
			return ast.ErrASTDomain
		} // Native NOT_IMPLEMENTED_YET.
		lengthTester = stack[len(stack)-1]
		literal, assigned, err := t.eqcStringValue(lengthTester)
		if err != nil {
			return err
		}
		if !assigned {
			return nil
		} // Native warning with no generated assertion.
		lengthText, _, err = t.state.manager.SequenceStringValue(literal)
		if err != nil {
			return err
		}
		if lengthText == "more" || lengthText == "less" {
			return ast.ErrASTDomain
		}
	} else {
		// This source scan deliberately has no active-membership filter.
		for _, indicator := range t.testers.lengthIndicators[word] {
			literal, assigned, err := t.eqcStringValue(indicator)
			if err != nil {
				return err
			}
			if !assigned {
				continue
			}
			text, _, err := t.state.manager.SequenceStringValue(literal)
			if err != nil {
				return err
			}
			if text != "more" {
				lengthTester, lengthText = indicator, text
				break
			}
		}
	}
	root, err := t.generateStringFreeValueOptions(word, lengthTester, lengthText, tester, value)
	if err != nil {
		return err
	}
	return t.assertStringTesterOptions(root)
}

func (t *SMTStringTheory) attemptStringFreeVariable(first, second ast.ASTHandle) (bool, error) {
	_, length := t.lengthTestVariables[first]
	_, value := t.valueTestVariables[first]
	if length || value {
		text, literal, err := t.state.manager.SequenceStringValue(second)
		if err != nil {
			return false, err
		}
		if literal {
			if length {
				return true, t.moreStringLengthTests(first, text)
			}
			if text == "more" {
				return true, t.moreStringValueTests(first, text)
			}
			return true, nil
		}
	}
	_, unroll := t.unrollTestVariables[first]
	return unroll, nil
}
