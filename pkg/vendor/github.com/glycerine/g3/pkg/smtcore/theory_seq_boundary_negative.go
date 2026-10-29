package smtcore

// Source: src/smt/theory_seq.cpp retained propagate_not_suffix.

import "github.com/glycerine/g3/pkg/ast"

func (t *SMTSequenceTheory) concatThree(first, second, third ast.ASTHandle) (ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	tail, err := state.manager.MkSequenceConcat(second, third)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return state.manager.MkSequenceConcat(first, tail)
}

func (t *SMTSequenceTheory) addNegativeSuffixWitness(expression, boundary, container ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	sort, err := state.manager.GetSort(boundary)
	if err != nil {
		return err
	}
	elementSort, sequence, err := state.manager.SequenceSortElement(sort)
	if err != nil || !sequence {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	prefix := "seq.suffix."
	arguments := []ast.ASTHandle{boundary, container}
	x, err := t.relationSkolem(prefix+"x", arguments, sort)
	if err != nil {
		return err
	}
	y, err := t.relationSkolem(prefix+"y", arguments, sort)
	if err != nil {
		return err
	}
	z, err := t.relationSkolem(prefix+"z", arguments, sort)
	if err != nil {
		return err
	}
	c, err := t.relationSkolem(prefix+"c", arguments, elementSort)
	if err != nil {
		return err
	}
	d, err := t.relationSkolem(prefix+"d", arguments, elementSort)
	if err != nil {
		return err
	}
	unitC, err := state.manager.MkSequenceUnit(c)
	if err != nil {
		return err
	}
	unitD, err := state.manager.MkSequenceUnit(d)
	if err != nil {
		return err
	}
	var boundaryWord, containerWord ast.ASTHandle
	boundaryWord, err = t.concatThree(y, unitC, x)
	if err == nil {
		containerWord, err = t.concatThree(z, unitD, x)
	}

	if err != nil {
		return err
	}
	boundaryDefinition, err := state.manager.MkEq(boundary, boundaryWord)
	if err != nil {
		return err
	}
	containerDefinition, err := state.manager.MkEq(container, containerWord)
	if err != nil {
		return err
	}
	containerAtShared, err := state.manager.MkEq(container, x)
	if err != nil {
		return err
	}
	elementsEqual, err := state.manager.MkEq(c, d)
	if err != nil {
		return err
	}
	elementsDistinct, err := state.manager.MkNot(elementsEqual)
	if err != nil {
		return err
	}
	empty, err := state.manager.MkSequenceEmpty(sort)
	if err != nil {
		return err
	}
	containerEmpty, err := state.manager.MkEq(container, empty)
	if err != nil {
		return err
	}
	containerNonempty, err := state.manager.MkNot(containerEmpty)
	if err != nil {
		return err
	}
	notRelation, err := state.manager.MkNot(expression)
	if err != nil {
		return err
	}
	conditions := []ast.ASTHandle{notRelation, containerNonempty}
	if err = t.pushSequenceImplication(conditions, boundaryDefinition); err != nil {
		return err
	}
	containerAlternative, err := state.manager.MkOr(containerDefinition, containerAtShared)
	if err != nil {
		return err
	}
	if err = t.pushSequenceImplication(conditions, containerAlternative); err != nil {
		return err
	}
	mismatchAlternative, err := state.manager.MkOr(elementsDistinct, containerAtShared)
	if err != nil {
		return err
	}
	return t.pushSequenceImplication(conditions, mismatchAlternative)
}
