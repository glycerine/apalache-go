package smtcore

// Source: src/smt/theory_seq.cpp add_at_axiom.

import "github.com/glycerine/g3/pkg/ast"

func (t *SMTSequenceTheory) pushSequenceImplication(conditions []ast.ASTHandle, conclusion ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	antecedent, err := state.manager.MkAndArray(conditions)
	if err != nil {
		return err
	}
	formula, err := state.manager.MkImplies(antecedent, conclusion)
	if err != nil {
		return err
	}
	return t.axioms.Push(formula)
}

func (t *SMTSequenceTheory) addAtAxioms(expression ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	sequence, err := state.manager.GetArg(expression, 0)
	if err != nil {
		return err
	}
	index, err := state.manager.GetArg(expression, 1)
	if err != nil {
		return err
	}
	sort, err := state.manager.GetSort(sequence)
	if err != nil {
		return err
	}
	zero, err := t.integer(0)
	if err != nil {
		return err
	}
	one, err := t.integer(1)
	if err != nil {
		return err
	}
	length, err := state.manager.MkSequenceLength(sequence)
	if err != nil {
		return err
	}
	nonnegative, err := state.manager.MkArithmeticGE(index, zero)
	if err != nil {
		return err
	}
	outOfBounds, err := state.manager.MkArithmeticGE(index, length)
	if err != nil {
		return err
	}
	inside, err := state.manager.MkNot(outOfBounds)
	if err != nil {
		return err
	}
	negative, err := state.manager.MkNot(nonnegative)
	if err != nil {
		return err
	}
	empty, err := state.manager.MkSequenceEmpty(sort)
	if err != nil {
		return err
	}
	emptyValue, err := state.manager.MkEq(expression, empty)
	if err != nil {
		return err
	}
	if err = t.pushSequenceImplication([]ast.ASTHandle{negative}, emptyValue); err != nil {
		return err
	}
	if err = t.pushSequenceImplication([]ast.ASTHandle{outOfBounds}, emptyValue); err != nil {
		return err
	}

	prefix, err := t.relationSkolem("seq.pre", []ast.ASTHandle{sequence, index}, sort)
	if err != nil {
		return err
	}
	suffix, err := t.relationSkolem("seq.post", []ast.ASTHandle{sequence, index}, sort)
	if err != nil {
		return err
	}
	tail, err := state.manager.MkSequenceConcat(expression, suffix)
	if err != nil {
		return err
	}
	word, err := state.manager.MkSequenceConcat(prefix, tail)
	if err != nil {
		return err
	}
	decomposition, err := state.manager.MkEq(sequence, word)
	if err != nil {
		return err
	}
	valid := []ast.ASTHandle{nonnegative, inside}
	if err = t.pushSequenceImplication(valid, decomposition); err != nil {
		return err
	}
	prefixLength, err := state.manager.MkSequenceLength(prefix)
	if err != nil {
		return err
	}
	prefixAtIndex, err := state.manager.MkEq(prefixLength, index)
	if err != nil {
		return err
	}
	if err = t.pushSequenceImplication(valid, prefixAtIndex); err != nil {
		return err
	}
	valueLength, err := state.manager.MkSequenceLength(expression)
	if err != nil {
		return err
	}
	unitLength, err := state.manager.MkEq(valueLength, one)
	if err != nil {
		return err
	}
	return t.pushSequenceImplication(valid, unitLength)
}
