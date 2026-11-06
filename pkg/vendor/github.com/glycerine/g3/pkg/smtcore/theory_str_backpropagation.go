package smtcore

// Source: src/smt/theory_str.cpp final_check_eh constant-backpropagation stage,
// collect_var_concat, get_unique_non_concat_nodes, propagate_length and
// propagate_length_within_eqc.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) backpropagateStringConstants(analysis *smtStringDependencyAnalysis) (added bool, result error) {
	m := t.state.manager
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	for _, variable := range stringOrderedRelations(analysis.variableConcats) {
		for _, concat := range stringOrderedCounts(analysis.variableConcats[variable]) {
			left, right, matched, err := m.SequenceConcatParts(concat)
			if err != nil {
				return added, err
			}
			if !matched {
				return added, ast.ErrASTDomain
			}
			leftValue, leftKnown, err := t.eqcStringValue(left)
			if err != nil {
				return added, err
			}
			rightValue, rightKnown, err := t.eqcStringValue(right)
			if err != nil {
				return added, err
			}
			_, variableKnown, err := t.eqcStringValue(variable)
			if err != nil {
				return added, err
			}
			if !leftKnown || !rightKnown || variableKnown {
				continue
			}
			leftText, _, err := m.SequenceStringValue(leftValue)
			if err != nil {
				return added, err
			}
			rightText, _, err := m.SequenceStringValue(rightValue)
			if err != nil {
				return added, err
			}
			premises := smtStringLengthPremises{theory: t}
			err = func() (result error) {
				defer func() { result = errors.Join(result, premises.close()) }()
				for _, pair := range [][2]ast.ASTHandle{{left, leftValue}, {right, rightValue}} {
					if _, literal, err := m.SequenceStringValue(pair[0]); err != nil {
						return err
					} else if !literal {
						if err := premises.equate(pair[0], pair[1]); err != nil {
							return err
						}
					}
				}
				value, err := t.stringLiteral(leftText + rightText)
				if err != nil {
					return err
				}
				if err := locals.add(value); err != nil {
					return err
				}
				conclusion, err := m.MkEq(concat, value)
				if err != nil {
					return err
				}
				return premises.imply(conclusion)
			}()
			if err != nil {
				return added, err
			}
			added = true
		}
	}
	return added, nil
}

func (t *SMTStringTheory) collectStringLengthExpressions(expression ast.ASTHandle, variables, concats map[ast.ASTHandle]int) error {
	m := t.state.manager
	work := []ast.ASTHandle{expression}
	for len(work) != 0 {
		last := len(work) - 1
		current := work[last]
		work = work[:last]
		if _, variable := t.variableSet[current]; variable {
			if _, tester := t.lengthTestVariables[current]; !tester {
				variables[current] = 1
			}
			continue
		}
		kind, err := m.GetKind(current)
		if err != nil {
			return err
		}
		if kind != ast.ASTApp {
			continue
		}
		op, own, err := t.operation(current)
		if err != nil {
			return err
		}
		if own && op == 9 {
			continue
		}
		if own && op == ast.ASTSequenceOpConcat {
			concats[current] = 1
		}
		count, err := m.GetNumArgs(current)
		if err != nil {
			return err
		}
		for i := count; i > 0; i-- {
			child, err := m.GetArg(current, i-1)
			if err != nil {
				return err
			}
			work = append(work, child)
		}
	}
	return nil
}

func (t *SMTStringTheory) uniqueStringConcatLeaves(expression ast.ASTHandle) ([]ast.ASTHandle, error) {
	set := make(map[ast.ASTHandle]int)
	work := []ast.ASTHandle{expression}
	for len(work) != 0 {
		last := len(work) - 1
		current := work[last]
		work = work[:last]
		left, right, concat, err := t.state.manager.SequenceConcatParts(current)
		if err != nil {
			return nil, err
		}
		if concat {
			work = append(work, right, left)
		} else {
			set[current] = 1
		}
	}
	return stringOrderedCounts(set), nil
}

func (t *SMTStringTheory) propagateStringLengthWithinClass(variable ast.ASTHandle) (added bool, result error) {
	if _, known, err := t.lengthValue(variable); err != nil || known {
		return false, err
	}
	m := t.state.manager
	for _, member := range t.stringClass(variable) {
		value, known, err := t.lengthValue(member)
		if err != nil {
			return false, err
		}
		if !known {
			continue
		}
		premises := smtStringLengthPremises{theory: t}
		defer func() { result = errors.Join(result, premises.close()) }()
		locals := smtStringLengthPremises{theory: t}
		defer func() { result = errors.Join(result, locals.close()) }()
		if err := premises.equate(variable, member); err != nil {
			return false, err
		}
		if err := premises.length(member, value); err != nil {
			return false, err
		}
		length, err := m.MkSequenceLength(variable)
		if err != nil {
			return false, err
		}
		if err := locals.add(length); err != nil {
			return false, err
		}
		// Retained C++ shadows rational varLen with expr_ref varLen here.
		// mk_int(varLen) selects mk_int(int) through obj_ref::operator bool,
		// so the emitted conclusion is length(variable)=1, not the numeral
		// from the premise. Preserve that source behavior explicitly.
		one, err := m.ArithmeticIntegerNumeralText("1")
		if err != nil {
			return false, err
		}
		if err := locals.add(one); err != nil {
			return false, err
		}
		conclusion, err := m.MkEq(length, one)
		if err != nil {
			return false, err
		}
		return true, premises.imply(conclusion)
	}
	return false, nil
}

func (t *SMTStringTheory) propagateStringLengths() (added bool, result error) {
	m := t.state.manager
	variables, concats := make(map[ast.ASTHandle]int), make(map[ast.ASTHandle]int)
	for _, literal := range t.state.assignedLiterals {
		if literal.Sign() {
			continue
		}
		expression, err := t.state.BoolExpression(literal.Var())
		if err != nil {
			return added, err
		}
		if !t.state.isRelevant(expression) {
			continue
		}
		op, basic, err := t.state.basicOperation(expression)
		if err != nil {
			return added, err
		}
		if basic && op == ast.ASTOpEq {
			if err := t.collectStringLengthExpressions(expression, variables, concats); err != nil {
				return added, err
			}
		}
	}
	for _, concat := range stringOrderedCounts(concats) {
		err := func() (result error) {
			locals := smtStringLengthPremises{theory: t}
			defer func() { result = errors.Join(result, locals.close()) }()
			length, err := m.MkSequenceLength(concat)
			if err != nil {
				return err
			}
			if err := locals.add(length); err != nil {
				return err
			}
			if _, known, err := t.arithmeticValue(length); err != nil || known {
				return err
			}
			value, known, err := t.lengthValue(concat)
			if err != nil || !known {
				return err
			}
			leaves, err := t.uniqueStringConcatLeaves(concat)
			if err != nil {
				return err
			}
			premises := smtStringLengthPremises{theory: t}
			defer func() { result = errors.Join(result, premises.close()) }()
			for _, leaf := range leaves {
				leafValue, known, err := t.lengthValue(leaf)
				if err != nil || !known {
					return err
				}
				if err := premises.length(leaf, leafValue); err != nil {
					return err
				}
			}
			numeral, err := m.ArithmeticIntegerNumeralText(value.String())
			if err != nil {
				return err
			}
			if err := locals.add(numeral); err != nil {
				return err
			}
			conclusion, err := m.MkEq(length, numeral)
			if err != nil {
				return err
			}
			if err := premises.imply(conclusion); err != nil {
				return err
			}
			added = true
			return nil
		}()
		if err != nil {
			return added, err
		}
	}
	if !added {
		for _, variable := range stringOrderedCounts(variables) {
			length, err := m.MkSequenceLength(variable)
			if err != nil {
				return added, err
			}
			root, err := m.NewRef(length)
			if err != nil {
				return added, err
			}
			_, known, err := t.arithmeticValue(length)
			if err == nil && !known {
				var changed bool
				changed, err = t.propagateStringLengthWithinClass(variable)
				added = added || changed
			}
			if err := errors.Join(err, m.DelRef(&root)); err != nil {
				return added, err
			}
		}
	}
	return added, nil
}
