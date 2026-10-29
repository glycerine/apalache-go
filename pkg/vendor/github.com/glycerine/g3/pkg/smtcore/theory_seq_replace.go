package smtcore

// Source: src/smt/theory_seq.cpp add_replace_axiom.

import "github.com/glycerine/g3/pkg/ast"

func (t *SMTSequenceTheory) addReplaceAxioms(expression ast.ASTHandle) error {
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
	replacement, err := state.manager.GetArg(expression, 2)
	if err != nil {
		return err
	}
	sort, err := state.manager.GetSort(source)
	if err != nil {
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
	notSourceEmpty, err := state.manager.MkNot(sourceEmpty)
	if err != nil {
		return err
	}
	notNeedleEmpty, err := state.manager.MkNot(needleEmpty)
	if err != nil {
		return err
	}
	unchanged, err := state.manager.MkEq(expression, source)
	if err != nil {
		return err
	}
	if err = t.pushSequenceImplication([]ast.ASTHandle{sourceEmpty, notNeedleEmpty}, unchanged); err != nil {
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
	if err = t.pushSequenceImplication([]ast.ASTHandle{notContains}, unchanged); err != nil {
		return err
	}
	emptyNeedleResult, err := state.manager.MkSequenceConcat(replacement, source)
	if err != nil {
		return err
	}
	emptyNeedleDefinition, err := state.manager.MkEq(expression, emptyNeedleResult)
	if err != nil {
		return err
	}
	if err = t.pushSequenceImplication([]ast.ASTHandle{needleEmpty}, emptyNeedleDefinition); err != nil {
		return err
	}

	arguments := []ast.ASTHandle{source, needle}
	prefix, err := t.relationSkolem("seq.idx.left", arguments, sort)
	if err != nil {
		return err
	}
	suffix, err := t.relationSkolem("seq.idx.right", arguments, sort)
	if err != nil {
		return err
	}
	sourceTail, err := state.manager.MkSequenceConcat(needle, suffix)
	if err != nil {
		return err
	}
	sourceWord, err := state.manager.MkSequenceConcat(prefix, sourceTail)
	if err != nil {
		return err
	}
	sourceDefinition, err := state.manager.MkEq(source, sourceWord)
	if err != nil {
		return err
	}
	resultTail, err := state.manager.MkSequenceConcat(replacement, suffix)
	if err != nil {
		return err
	}
	resultWord, err := state.manager.MkSequenceConcat(prefix, resultTail)
	if err != nil {
		return err
	}
	resultDefinition, err := state.manager.MkEq(expression, resultWord)
	if err != nil {
		return err
	}
	found := []ast.ASTHandle{contains, notSourceEmpty, notNeedleEmpty}
	if err = t.pushSequenceImplication(found, sourceDefinition); err != nil {
		return err
	}
	if err = t.pushSequenceImplication(found, resultDefinition); err != nil {
		return err
	}
	return t.addTightestPrefixAxioms(prefix, needle)
}
