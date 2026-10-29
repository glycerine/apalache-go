package smtcore

// Source: src/smt/theory_seq.cpp add_indexof_axiom.

import "github.com/glycerine/g3/pkg/ast"

func (t *SMTSequenceTheory) addTightestPrefixAxioms(prefix, needle ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	sort, err := state.manager.GetSort(prefix)
	if err != nil {
		return err
	}
	elementSort, sequence, err := state.manager.SequenceSortElement(sort)
	if err != nil {
		return err
	}
	if !sequence {
		return ast.ErrASTDomain
	}
	empty, err := state.manager.MkSequenceEmpty(sort)
	if err != nil {
		return err
	}
	prefixEmpty, err := state.manager.MkEq(prefix, empty)
	if err != nil {
		return err
	}
	prefixNonempty, err := state.manager.MkNot(prefixEmpty)
	if err != nil {
		return err
	}
	initial, err := t.relationSkolem("seq.first", []ast.ASTHandle{prefix}, sort)
	if err != nil {
		return err
	}
	last, err := t.relationSkolem("seq.last", []ast.ASTHandle{prefix}, elementSort)
	if err != nil {
		return err
	}
	unit, err := state.manager.MkSequenceUnit(last)
	if err != nil {
		return err
	}
	decomposition, err := state.manager.MkSequenceConcat(initial, unit)
	if err != nil {
		return err
	}
	prefixDefinition, err := state.manager.MkEq(prefix, decomposition)
	if err != nil {
		return err
	}
	if err = t.pushSequenceImplication([]ast.ASTHandle{prefixNonempty}, prefixDefinition); err != nil {
		return err
	}
	needleAndInitial, err := state.manager.MkSequenceConcat(needle, initial)
	if err != nil {
		return err
	}
	prematureMatch, err := state.manager.MkSequenceContains(needleAndInitial, prefix)
	if err != nil {
		return err
	}
	notPrematureMatch, err := state.manager.MkNot(prematureMatch)
	if err != nil {
		return err
	}
	t.trackInternalTerms(initial, last, unit, decomposition, needleAndInitial, prematureMatch)
	return t.pushSequenceImplication([]ast.ASTHandle{prefixNonempty}, notPrematureMatch)
}

func (t *SMTSequenceTheory) addIndexAxioms(expression ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	source, err := state.manager.GetArg(expression, 0)
	if err != nil {
		return err
	}
	needle, err := state.manager.GetArg(expression, 1)
	if err != nil {
		return err
	}
	offset, err := state.manager.GetArg(expression, 2)
	if err != nil {
		return err
	}
	sort, err := state.manager.GetSort(source)
	if err != nil {
		return err
	}
	zero, err := t.integer(0)
	if err != nil {
		return err
	}
	minusOne, err := state.manager.ArithmeticIntegerNumeralText("-1")
	if err != nil {
		return err
	}
	indexMinusOne, err := state.manager.MkEq(expression, minusOne)
	if err != nil {
		return err
	}
	contains, err := state.manager.MkSequenceContains(source, needle)
	if err != nil {
		return err
	}
	notContains, err := state.manager.MkNot(contains)
	if err != nil {
		return err
	}
	if err = t.pushSequenceImplication([]ast.ASTHandle{notContains}, indexMinusOne); err != nil {
		return err
	}
	empty, err := state.manager.MkSequenceEmpty(sort)
	if err != nil {
		return err
	}
	sourceEmpty, err := state.manager.MkEq(source, empty)
	if err != nil {
		return err
	}
	needleEmpty, err := state.manager.MkEq(needle, empty)
	if err != nil {
		return err
	}
	notNeedleEmpty, err := state.manager.MkNot(needleEmpty)
	if err != nil {
		return err
	}
	if err = t.pushSequenceImplication([]ast.ASTHandle{sourceEmpty, notNeedleEmpty}, indexMinusOne); err != nil {
		return err
	}

	offsetText, integer, numeral, err := state.manager.ArithmeticNumeralText(offset)
	if err != nil {
		return err
	}
	if numeral && integer && offsetText == "0" {
		indexZero, makeErr := state.manager.MkEq(expression, zero)
		if makeErr != nil {
			return makeErr
		}
		if makeErr = t.pushSequenceImplication([]ast.ASTHandle{needleEmpty}, indexZero); makeErr != nil {
			return makeErr
		}
		arguments := []ast.ASTHandle{source, needle}
		prefix, makeErr := t.relationSkolem("seq.idx.left", arguments, sort)
		if makeErr != nil {
			return makeErr
		}
		suffix, makeErr := t.relationSkolem("seq.idx.right", arguments, sort)
		if makeErr != nil {
			return makeErr
		}
		tail, makeErr := state.manager.MkSequenceConcat(needle, suffix)
		if makeErr != nil {
			return makeErr
		}
		word, makeErr := state.manager.MkSequenceConcat(prefix, tail)
		if makeErr != nil {
			return makeErr
		}
		decomposition, makeErr := state.manager.MkEq(source, word)
		if makeErr != nil {
			return makeErr
		}
		found := []ast.ASTHandle{contains, notNeedleEmpty}
		if makeErr = t.pushSequenceImplication(found, decomposition); makeErr != nil {
			return makeErr
		}
		prefixLength, makeErr := state.manager.MkSequenceLength(prefix)
		if makeErr != nil {
			return makeErr
		}
		indexLength, makeErr := state.manager.MkEq(expression, prefixLength)
		if makeErr != nil {
			return makeErr
		}
		if makeErr = t.pushSequenceImplication(found, indexLength); makeErr != nil {
			return makeErr
		}
		nonnegative, makeErr := state.manager.MkArithmeticGE(expression, zero)
		if makeErr != nil {
			return makeErr
		}
		if makeErr = t.pushSequenceImplication([]ast.ASTHandle{contains}, nonnegative); makeErr != nil {
			return makeErr
		}
		return t.addTightestPrefixAxioms(prefix, needle)
	}

	sourceLength, err := state.manager.MkSequenceLength(source)
	if err != nil {
		return err
	}
	offsetNonnegative, err := state.manager.MkArithmeticGE(offset, zero)
	if err != nil {
		return err
	}
	offsetNegative, err := state.manager.MkNot(offsetNonnegative)
	if err != nil {
		return err
	}
	if err = t.pushSequenceImplication([]ast.ASTHandle{offsetNegative}, indexMinusOne); err != nil {
		return err
	}
	offsetAtLeastLength, err := state.manager.MkArithmeticGE(offset, sourceLength)
	if err != nil {
		return err
	}
	offsetAtMostLength, err := state.manager.MkArithmeticLE(offset, sourceLength)
	if err != nil {
		return err
	}
	offsetPastLength, err := state.manager.MkNot(offsetAtMostLength)
	if err != nil {
		return err
	}
	if err = t.pushSequenceImplication([]ast.ASTHandle{offsetPastLength}, indexMinusOne); err != nil {
		return err
	}
	if err = t.pushSequenceImplication([]ast.ASTHandle{offsetAtLeastLength, notNeedleEmpty}, indexMinusOne); err != nil {
		return err
	}
	offsetEqualsLength, err := state.manager.MkAnd(offsetAtLeastLength, offsetAtMostLength)
	if err != nil {
		return err
	}
	indexOffset, err := state.manager.MkEq(expression, offset)
	if err != nil {
		return err
	}
	if err = t.pushSequenceImplication([]ast.ASTHandle{offsetEqualsLength, needleEmpty}, indexOffset); err != nil {
		return err
	}

	beforeEnd, err := state.manager.MkNot(offsetAtLeastLength)
	if err != nil {
		return err
	}
	valid := []ast.ASTHandle{offsetNonnegative, beforeEnd}
	arguments := []ast.ASTHandle{source, needle, offset}
	prefix, err := t.relationSkolem("seq.idx.left", arguments, sort)
	if err != nil {
		return err
	}
	suffix, err := t.relationSkolem("seq.idx.right", arguments, sort)
	if err != nil {
		return err
	}
	word, err := state.manager.MkSequenceConcat(prefix, suffix)
	if err != nil {
		return err
	}
	decomposition, err := state.manager.MkEq(source, word)
	if err != nil {
		return err
	}
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
	restIndex, err := state.manager.MkSequenceIndex(suffix, needle, zero)
	if err != nil {
		return err
	}
	restMissing, err := state.manager.MkEq(restIndex, minusOne)
	if err != nil {
		return err
	}
	if err = t.pushSequenceImplication(append(valid, restMissing), indexMinusOne); err != nil {
		return err
	}
	restNonnegative, err := state.manager.MkArithmeticGE(restIndex, zero)
	if err != nil {
		return err
	}
	translated, err := state.manager.MkArithmeticAdd(offset, restIndex)
	if err != nil {
		return err
	}
	translatedIndex, err := state.manager.MkEq(expression, translated)
	if err != nil {
		return err
	}
	return t.pushSequenceImplication(append(valid, restNonnegative), translatedIndex)
}
