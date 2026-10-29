package smtcore

// Source: src/smt/theory_seq.cpp is_tail, is_drop_last, add_tail_axiom and
// add_drop_last_axiom.

import "github.com/glycerine/g3/pkg/ast"

func (t *SMTSequenceTheory) isLengthMinusOne(expression, length ast.ASTHandle) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	one, err := t.integer(1)
	if err != nil {
		return false, err
	}
	difference, err := state.manager.MkArithmeticSub(length, one)
	if err != nil || expression == difference {
		return expression == difference, err
	}
	family, err := state.manager.GetFamilyID(expression)
	if err != nil {
		return false, err
	}
	arithmeticFamily, err := state.manager.ArithmeticFamilyID()
	if err != nil || family != arithmeticFamily {
		return false, err
	}
	kind, err := state.manager.GetDeclKind(expression)
	if err != nil || kind != smtArithOpAdd {
		return false, err
	}
	left, err := state.manager.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	right, err := state.manager.GetArg(expression, 1)
	if err != nil {
		return false, err
	}
	for _, pair := range [][2]ast.ASTHandle{{left, right}, {right, left}} {
		text, integer, numeral, numeralErr := state.manager.ArithmeticNumeralText(pair[0])
		if numeralErr != nil {
			return false, numeralErr
		}
		if integer && numeral && text == "-1" && pair[1] == length {
			return true, nil
		}
	}
	return false, nil
}

func (t *SMTSequenceTheory) addSpecialExtractAxioms(expression, sequence, offset, requested, sequenceLength, empty ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	lengthMinusOne, err := t.isLengthMinusOne(requested, sequenceLength)
	if err != nil || !lengthMinusOne {
		return err
	}
	offsetText, offsetInteger, offsetNumeral, err := state.manager.ArithmeticNumeralText(offset)
	if err != nil || !offsetInteger || !offsetNumeral || offsetText != "0" && offsetText != "1" {
		return err
	}
	sort, err := state.manager.GetSort(sequence)
	if err != nil {
		return err
	}
	elementSort, isSequence, err := state.manager.SequenceSortElement(sort)
	if err != nil || !isSequence {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	var word, witness ast.ASTHandle
	if offsetText == "1" {
		zero, makeErr := t.integer(0)
		if makeErr != nil {
			return makeErr
		}
		head, makeErr := t.relationSkolem("seq.nth", []ast.ASTHandle{sequence, zero}, elementSort)
		if makeErr != nil {
			return makeErr
		}
		unit, makeErr := state.manager.MkSequenceUnit(head)
		if makeErr != nil {
			return makeErr
		}
		word, err = state.manager.MkSequenceConcat(unit, expression)
		witness, makeErr = t.relationSkolem("seq.tail", []ast.ASTHandle{sequence, zero}, sort)
		if makeErr != nil {
			return makeErr
		}
		t.trackInternalTerms(head, unit, word, witness)
	} else {
		last, makeErr := t.relationSkolem("seq.last", []ast.ASTHandle{sequence}, elementSort)
		if makeErr != nil {
			return makeErr
		}
		unit, makeErr := state.manager.MkSequenceUnit(last)
		if makeErr != nil {
			return makeErr
		}
		word, err = state.manager.MkSequenceConcat(expression, unit)
		witness, makeErr = t.relationSkolem("seq.first", []ast.ASTHandle{sequence}, sort)
		if makeErr != nil {
			return makeErr
		}
		t.trackInternalTerms(last, unit, word, witness)
	}
	if err != nil {
		return err
	}
	definition, err := state.manager.MkEq(sequence, word)
	if err != nil {
		return err
	}
	emptySequence, err := state.manager.MkEq(sequence, empty)
	if err != nil {
		return err
	}
	nonempty, err := state.manager.MkNot(emptySequence)
	if err != nil {
		return err
	}
	witnessDefinition, err := state.manager.MkEq(expression, witness)
	if err != nil {
		return err
	}
	if err = t.pushSequenceImplication([]ast.ASTHandle{nonempty}, witnessDefinition); err != nil {
		return err
	}
	return t.pushSequenceImplication([]ast.ASTHandle{nonempty}, definition)
}
