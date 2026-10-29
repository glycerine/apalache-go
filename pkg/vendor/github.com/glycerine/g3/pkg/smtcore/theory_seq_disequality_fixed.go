package smtcore

// Source: src/smt/theory_seq.cpp solve_ne retained reduction of fixed-length
// sequence disequalities to element disequalities.

import "github.com/glycerine/g3/pkg/ast"

func (t *SMTSequenceTheory) fixedDisequalityElements(expression ast.ASTHandle) ([]ast.ASTHandle, ast.ASTHandle, bool, error) {
	state, err := t.State()
	if err != nil {
		return nil, ast.ASTHandle{}, false, err
	}
	if text, concrete, textErr := state.manager.SequenceStringValue(expression); textErr != nil {
		return nil, ast.ASTHandle{}, false, textErr
	} else if concrete {
		elements := make([]ast.ASTHandle, 0, len(text))
		for index := 0; index < len(text); index++ {
			element, makeErr := state.manager.BitVectorNumeralUnsigned(uint32(text[index]), 8)
			if makeErr != nil {
				return nil, ast.ASTHandle{}, false, makeErr
			}
			elements = append(elements, element)
		}
		truth, makeErr := state.manager.MkTrue()
		return elements, truth, true, makeErr
	}
	sort, err := state.manager.GetSort(expression)
	if err != nil {
		return nil, ast.ASTHandle{}, false, err
	}
	elementSort, sequence, err := state.manager.SequenceSortElement(sort)
	if err != nil {
		return nil, ast.ASTHandle{}, false, err
	}
	if !sequence {
		return nil, ast.ASTHandle{}, false, nil
	}
	count, exact, err := t.exactSequenceLength(expression)
	if err != nil || !exact || count > smtSequenceMaximumFixedDecomposition {
		return nil, ast.ASTHandle{}, false, err
	}
	elements := make([]ast.ASTHandle, 0, int(count))
	for index := uint64(0); index < count; index++ {
		position, makeErr := t.integer(index)
		if makeErr != nil {
			return nil, ast.ASTHandle{}, false, makeErr
		}
		element, makeErr := t.relationSkolem("seq.nth", []ast.ASTHandle{expression, position}, elementSort)
		if makeErr != nil {
			return nil, ast.ASTHandle{}, false, makeErr
		}
		elements = append(elements, element)
	}
	length, err := state.manager.MkSequenceLength(expression)
	if err != nil {
		return nil, ast.ASTHandle{}, false, err
	}
	lengthValue, err := t.integer(count)
	if err != nil {
		return nil, ast.ASTHandle{}, false, err
	}
	guard, err := state.manager.MkEq(length, lengthValue)
	if err != nil {
		return nil, ast.ASTHandle{}, false, err
	}
	t.trackInternalTerms(elements...)
	return elements, guard, true, nil
}

func (t *SMTSequenceTheory) addFixedLengthDisequalityReduction() (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	for index := 0; index < t.disequations.Len(); index++ {
		left, right, _, _, literals, _, entryErr := t.disequations.At(index)
		if entryErr != nil {
			return false, entryErr
		}
		if len(literals) != 0 {
			continue
		}
		leftElements, leftGuard, leftExact, entryErr := t.fixedDisequalityElements(left)
		if entryErr != nil {
			return false, entryErr
		}
		rightElements, rightGuard, rightExact, entryErr := t.fixedDisequalityElements(right)
		if entryErr != nil {
			return false, entryErr
		}
		if !leftExact || !rightExact || len(leftElements) != len(rightElements) {
			continue
		}
		disjuncts := make([]ast.ASTHandle, 0, len(leftElements))
		for elementIndex := range leftElements {
			equality, makeErr := state.manager.MkEq(leftElements[elementIndex], rightElements[elementIndex])
			if makeErr != nil {
				return false, makeErr
			}
			disequality, makeErr := state.manager.MkNot(equality)
			if makeErr != nil {
				return false, makeErr
			}
			disjuncts = append(disjuncts, disequality)
		}
		elementDisequality, makeErr := state.manager.MkOrArray(disjuncts)
		if makeErr != nil {
			return false, makeErr
		}
		originalEquality, makeErr := state.manager.MkEq(left, right)
		if makeErr != nil {
			return false, makeErr
		}
		originalDisequality, makeErr := state.manager.MkNot(originalEquality)
		if makeErr != nil {
			return false, makeErr
		}
		antecedent, makeErr := state.manager.MkAndArray([]ast.ASTHandle{originalDisequality, leftGuard, rightGuard})
		if makeErr != nil {
			return false, makeErr
		}
		constraint, makeErr := state.manager.MkImplies(antecedent, elementDisequality)
		if makeErr != nil {
			return false, makeErr
		}
		if _, found := t.axioms.set[constraint]; found {
			continue
		}
		return true, t.axioms.Push(constraint)
	}
	return false, nil
}
