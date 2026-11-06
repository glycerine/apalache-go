package smtcore

// Source: src/smt/theory_seq.cpp add_extract_axiom and its general
// substring decomposition case.

import "github.com/glycerine/g3/pkg/ast"

func (t *SMTSequenceTheory) addExtractAxioms(expression ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	sequence, offset, requested, matched, err := state.manager.SequenceExtractArguments(expression)
	if err != nil || !matched {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	sort, err := state.manager.GetSort(sequence)
	if err != nil {
		return err
	}
	zero, err := t.integer(0)
	if err != nil {
		return err
	}
	sequenceLength, err := state.manager.MkSequenceLength(sequence)
	if err != nil {
		return err
	}
	resultLength, err := state.manager.MkSequenceLength(expression)
	if err != nil {
		return err
	}
	nonnegativeOffset, err := state.manager.MkArithmeticGE(offset, zero)
	if err != nil {
		return err
	}
	offsetPastEnd, err := state.manager.MkArithmeticGE(offset, sequenceLength)
	if err != nil {
		return err
	}
	beforeEnd, err := state.manager.MkNot(offsetPastEnd)
	if err != nil {
		return err
	}
	nonpositiveLength, err := state.manager.MkArithmeticLE(requested, zero)
	if err != nil {
		return err
	}
	positiveLength, err := state.manager.MkNot(nonpositiveLength)
	if err != nil {
		return err
	}
	empty, err := state.manager.MkSequenceEmpty(sort)
	if err != nil {
		return err
	}
	emptyResult, err := state.manager.MkEq(expression, empty)
	if err != nil {
		return err
	}
	negativeOffset, err := state.manager.MkNot(nonnegativeOffset)
	if err != nil {
		return err
	}
	for _, boundary := range []ast.ASTHandle{negativeOffset, offsetPastEnd, nonpositiveLength} {
		if err = t.pushSequenceImplication([]ast.ASTHandle{boundary}, emptyResult); err != nil {
			return err
		}
	}
	if err = t.addSpecialExtractAxioms(expression, sequence, offset, requested, sequenceLength, empty); err != nil {
		return err
	}

	remaining, err := state.manager.MkArithmeticSub(sequenceLength, offset)
	if err != nil {
		return err
	}
	postLength, err := state.manager.MkArithmeticSub(remaining, requested)
	if err != nil {
		return err
	}
	prefix, err := t.relationSkolem("seq.pre", []ast.ASTHandle{sequence, offset}, sort)
	if err != nil {
		return err
	}
	suffix, err := t.relationSkolem("seq.post", []ast.ASTHandle{sequence, postLength}, sort)
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
	valid := []ast.ASTHandle{nonnegativeOffset, beforeEnd, positiveLength}
	if err = t.pushSequenceImplication(valid, decomposition); err != nil {
		return err
	}
	prefixLength, err := state.manager.MkSequenceLength(prefix)
	if err != nil {
		return err
	}
	prefixAtOffset, err := state.manager.MkEq(prefixLength, offset)
	if err != nil {
		return err
	}
	if err = t.pushSequenceImplication(valid, prefixAtOffset); err != nil {
		return err
	}
	withinRemaining, err := state.manager.MkArithmeticLE(requested, remaining)
	if err != nil {
		return err
	}
	exactLength, err := state.manager.MkEq(resultLength, requested)
	if err != nil {
		return err
	}
	if err = t.pushSequenceImplication(append(valid, withinRemaining), exactLength); err != nil {
		return err
	}
	pastRemaining, err := state.manager.MkNot(withinRemaining)
	if err != nil {
		return err
	}
	truncatedLength, err := state.manager.MkEq(resultLength, remaining)
	if err != nil {
		return err
	}
	if err = t.pushSequenceImplication(append(valid, pastRemaining), truncatedLength); err != nil {
		return err
	}
	offsetText, offsetInteger, offsetNumeral, err := state.manager.ArithmeticNumeralText(offset)
	if err != nil || !offsetNumeral || !offsetInteger || offsetText != "0" {
		return err
	}
	requestedNonnegative, err := state.manager.MkArithmeticGE(requested, zero)
	if err != nil {
		return err
	}
	prefixDefinition, err := state.manager.MkSequenceConcat(expression, suffix)
	if err != nil {
		return err
	}
	prefixDecomposition, err := state.manager.MkEq(sequence, prefixDefinition)
	if err != nil {
		return err
	}
	prefixValid := []ast.ASTHandle{requestedNonnegative, withinRemaining}
	if err = t.pushSequenceImplication(prefixValid, prefixDecomposition); err != nil {
		return err
	}
	if err = t.pushSequenceImplication(prefixValid, exactLength); err != nil {
		return err
	}
	suffixLength, err := state.manager.MkSequenceLength(suffix)
	if err != nil {
		return err
	}
	suffixExact, err := state.manager.MkEq(suffixLength, postLength)
	if err != nil {
		return err
	}
	if err = t.pushSequenceImplication(prefixValid, suffixExact); err != nil {
		return err
	}
	fullSequence, err := state.manager.MkEq(expression, sequence)
	if err != nil {
		return err
	}
	return t.pushSequenceImplication([]ast.ASTHandle{pastRemaining}, fullSequence)
}
