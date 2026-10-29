package smtcore

// Source: src/smt/theory_seq.cpp is_binary_eq, solve_binary_eq and
// branch_binary_variable for fixed string boundaries.

import (
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTSequenceTheory) branchBinaryUnitEquation(left, right []ast.ASTHandle) (bool, error) {
	for pass := 0; pass < 2; pass++ {
		if len(left) == 2 && len(right) == 2 {
			leftVariable, err := t.sequenceVariable(left[0])
			if err != nil {
				return false, err
			}
			rightVariable, err := t.sequenceVariable(right[1])
			if err != nil {
				return false, err
			}
			state, stateErr := t.State()
			if stateErr != nil {
				return false, stateErr
			}
			leftText, leftString, textErr := state.manager.SequenceStringValue(left[1])
			if textErr != nil {
				return false, textErr
			}
			rightText, rightString, textErr := state.manager.SequenceStringValue(right[0])
			if textErr != nil {
				return false, textErr
			}
			if leftVariable && rightVariable && left[0] != right[1] && leftString && leftText != "" && rightString && rightText != "" {
				sort, sortErr := state.manager.GetSort(left[0])
				if sortErr != nil {
					return false, sortErr
				}
				tail, makeErr := t.relationSkolem("seq.binary.tail", []ast.ASTHandle{left[0], right[1], left[1], right[0]}, sort)
				if makeErr != nil {
					return false, makeErr
				}
				leftValue, makeErr := state.manager.MkSequenceConcat(right[0], tail)
				if makeErr != nil {
					return false, makeErr
				}
				rightValue, makeErr := state.manager.MkSequenceConcat(tail, left[1])
				if makeErr != nil {
					return false, makeErr
				}
				leftEquality, makeErr := state.manager.MkEq(left[0], leftValue)
				if makeErr != nil {
					return false, makeErr
				}
				rightEquality, makeErr := state.manager.MkEq(right[1], rightValue)
				if makeErr != nil {
					return false, makeErr
				}
				longAlternative, makeErr := state.manager.MkAnd(leftEquality, rightEquality)
				if makeErr != nil {
					return false, makeErr
				}
				choices := []ast.ASTHandle{longAlternative}
				for split := 0; split < len(rightText); split++ {
					remaining := rightText[split:]
					if !strings.HasPrefix(leftText, remaining) {
						continue
					}
					leftPrefix, prefixErr := state.manager.MkString(rightText[:split])
					if prefixErr != nil {
						return false, prefixErr
					}
					rightSuffix, prefixErr := state.manager.MkString(leftText[len(remaining):])
					if prefixErr != nil {
						return false, prefixErr
					}
					shortLeft, prefixErr := state.manager.MkEq(left[0], leftPrefix)
					if prefixErr != nil {
						return false, prefixErr
					}
					shortRight, prefixErr := state.manager.MkEq(right[1], rightSuffix)
					if prefixErr != nil {
						return false, prefixErr
					}
					shortAlternative, prefixErr := state.manager.MkAnd(shortLeft, shortRight)
					if prefixErr != nil {
						return false, prefixErr
					}
					choices = append(choices, shortAlternative)
				}
				alternatives, makeErr := state.manager.MkOrArray(choices)
				if makeErr != nil {
					return false, makeErr
				}
				leftWord, makeErr := t.makeWord(left, sort)
				if makeErr != nil {
					return false, makeErr
				}
				rightWord, makeErr := t.makeWord(right, sort)
				if makeErr != nil {
					return false, makeErr
				}
				equality, makeErr := state.manager.MkEq(leftWord, rightWord)
				if makeErr != nil {
					return false, makeErr
				}
				constraint, makeErr := state.manager.MkImplies(equality, alternatives)
				if makeErr != nil {
					return false, makeErr
				}
				if _, found := t.axioms.set[constraint]; found {
					return false, nil
				}
				return true, t.axioms.Push(constraint)
			}
		}
		left, right = right, left
	}
	return false, nil
}

func (t *SMTSequenceTheory) rejectImpossibleCyclicBinaryEquation(left, right []ast.ASTHandle, dependency *smtSequenceDependency) (bool, error) {
	for pass := 0; pass < 2; pass++ {
		if len(left) == 2 && len(right) == 2 && left[0] == right[1] {
			variable, err := t.sequenceVariable(left[0])
			if err != nil {
				return false, err
			}
			state, stateErr := t.State()
			if stateErr != nil {
				return false, stateErr
			}
			leftText, leftString, textErr := state.manager.SequenceStringValue(left[1])
			if textErr != nil {
				return false, textErr
			}
			rightText, rightString, textErr := state.manager.SequenceStringValue(right[0])
			if textErr != nil {
				return false, textErr
			}
			if variable && leftString && rightString && (len(leftText) != len(rightText) || !strings.Contains(leftText+leftText, rightText)) {
				return true, t.setDependencyConflict(dependency)
			}
		}
		left, right = right, left
	}
	return false, nil
}

func (t *SMTSequenceTheory) constrainCyclicUnitEquation(left, right []ast.ASTHandle, dependency *smtSequenceDependency) (bool, error) {
	for pass := 0; pass < 2; pass++ {
		if len(left) > 1 && len(right) > 1 && left[0] == right[len(right)-1] {
			variable, err := t.sequenceVariable(left[0])
			if err != nil {
				return false, err
			}
			if !variable {
				left, right = right, left
				continue
			}
			leftUnits := left[1:]
			rightUnits := right[:len(right)-1]
			state, stateErr := t.State()
			if stateErr != nil {
				return false, stateErr
			}
			leftElements := make([]ast.ASTHandle, len(leftUnits))
			rightElements := make([]ast.ASTHandle, len(rightUnits))
			allUnits := true
			for partIndex, parts := range [][]ast.ASTHandle{leftUnits, rightUnits} {
				elements := leftElements
				if partIndex != 0 {
					elements = rightElements
				}
				for index, part := range parts {
					element, unit, unitErr := state.manager.SequenceUnitArgument(part)
					if unitErr != nil {
						return false, unitErr
					}
					if !unit {
						allUnits = false
						break
					}
					elements[index] = element
				}
			}
			if !allUnits {
				left, right = right, left
				continue
			}
			if len(leftUnits) != len(rightUnits) {
				return true, t.setDependencyConflict(dependency)
			}
			alignments := make([]ast.ASTHandle, 0, len(leftUnits))
			for offset := 0; offset < len(leftUnits); offset++ {
				equalities := make([]ast.ASTHandle, 0, len(leftUnits))
				for index, element := range leftElements {
					equality, makeErr := state.manager.MkEq(element, rightElements[(offset+index)%len(rightElements)])
					if makeErr != nil {
						return false, makeErr
					}
					equalities = append(equalities, equality)
				}
				alignment, makeErr := state.manager.MkAndArray(equalities)
				if makeErr != nil {
					return false, makeErr
				}
				alignments = append(alignments, alignment)
			}
			compatible, makeErr := state.manager.MkOrArray(alignments)
			if makeErr != nil {
				return false, makeErr
			}
			sort, makeErr := state.manager.GetSort(left[0])
			if makeErr != nil {
				return false, makeErr
			}
			leftWord, makeErr := t.makeWord(left, sort)
			if makeErr != nil {
				return false, makeErr
			}
			rightWord, makeErr := t.makeWord(right, sort)
			if makeErr != nil {
				return false, makeErr
			}
			equality, makeErr := state.manager.MkEq(leftWord, rightWord)
			if makeErr != nil {
				return false, makeErr
			}
			constraint, makeErr := state.manager.MkImplies(equality, compatible)
			if makeErr != nil {
				return false, makeErr
			}
			if _, found := t.axioms.set[constraint]; found {
				return false, nil
			}
			return true, t.axioms.Push(constraint)
		}
		left, right = right, left
	}
	return false, nil
}

func (t *SMTSequenceTheory) branchVariableFixedUnitWordEquation(left, right []ast.ASTHandle) (bool, error) {
	for pass := 0; pass < 2; pass++ {
		if len(left) > 1 && len(right) != 0 {
			firstVariable, err := t.sequenceVariable(left[0])
			if err != nil {
				return false, err
			}
			state, stateErr := t.State()
			if stateErr != nil {
				return false, stateErr
			}
			allUnits := true
			for _, part := range right {
				if _, unit, unitErr := state.manager.SequenceUnitArgument(part); unitErr != nil {
					return false, unitErr
				} else if !unit {
					allUnits = false
					break
				}
			}
			if firstVariable && allUnits {
				sort, makeErr := state.manager.GetSort(left[0])
				if makeErr != nil {
					return false, makeErr
				}
				restWord, makeErr := t.makeWord(left[1:], sort)
				if makeErr != nil {
					return false, makeErr
				}
				alternatives := make([]ast.ASTHandle, 0, len(right)+1)
				for split := 0; split <= len(right); split++ {
					prefix, wordErr := t.makeWord(right[:split], sort)
					if wordErr != nil {
						return false, wordErr
					}
					suffix, wordErr := t.makeWord(right[split:], sort)
					if wordErr != nil {
						return false, wordErr
					}
					firstEquality, makeErr := state.manager.MkEq(left[0], prefix)
					if makeErr != nil {
						return false, makeErr
					}
					restEquality, makeErr := state.manager.MkEq(restWord, suffix)
					if makeErr != nil {
						return false, makeErr
					}
					alternative, makeErr := state.manager.MkAnd(firstEquality, restEquality)
					if makeErr != nil {
						return false, makeErr
					}
					alternatives = append(alternatives, alternative)
				}
				choices, makeErr := state.manager.MkOrArray(alternatives)
				if makeErr != nil {
					return false, makeErr
				}
				leftWord, makeErr := t.makeWord(left, sort)
				if makeErr != nil {
					return false, makeErr
				}
				rightWord, makeErr := t.makeWord(right, sort)
				if makeErr != nil {
					return false, makeErr
				}
				equality, makeErr := state.manager.MkEq(leftWord, rightWord)
				if makeErr != nil {
					return false, makeErr
				}
				constraint, makeErr := state.manager.MkImplies(equality, choices)
				if makeErr != nil {
					return false, makeErr
				}
				if _, found := t.axioms.set[constraint]; found {
					return false, nil
				}
				return true, t.axioms.Push(constraint)
			}
		}
		left, right = right, left
	}
	return false, nil
}

func (t *SMTSequenceTheory) branchVariableFixedWordEquation(left, right []ast.ASTHandle) (bool, error) {
	for pass := 0; pass < 2; pass++ {
		if len(left) > 1 && len(right) == 1 {
			firstVariable, err := t.sequenceVariable(left[0])
			if err != nil {
				return false, err
			}
			state, stateErr := t.State()
			if stateErr != nil {
				return false, stateErr
			}
			word, fixed, textErr := state.manager.SequenceStringValue(right[0])
			if textErr != nil {
				return false, textErr
			}
			if firstVariable && fixed {
				sort, makeErr := state.manager.GetSort(left[0])
				if makeErr != nil {
					return false, makeErr
				}
				restWord, makeErr := t.makeWord(left[1:], sort)
				if makeErr != nil {
					return false, makeErr
				}
				alternatives := make([]ast.ASTHandle, 0, len(word)+1)
				for split := 0; split <= len(word); split++ {
					prefix, makeErr := state.manager.MkString(word[:split])
					if makeErr != nil {
						return false, makeErr
					}
					suffix, makeErr := state.manager.MkString(word[split:])
					if makeErr != nil {
						return false, makeErr
					}
					firstEquality, makeErr := state.manager.MkEq(left[0], prefix)
					if makeErr != nil {
						return false, makeErr
					}
					secondEquality, makeErr := state.manager.MkEq(restWord, suffix)
					if makeErr != nil {
						return false, makeErr
					}
					alternative, makeErr := state.manager.MkAnd(firstEquality, secondEquality)
					if makeErr != nil {
						return false, makeErr
					}
					alternatives = append(alternatives, alternative)
				}
				choices, makeErr := state.manager.MkOrArray(alternatives)
				if makeErr != nil {
					return false, makeErr
				}
				leftWord, makeErr := t.makeWord(left, sort)
				if makeErr != nil {
					return false, makeErr
				}
				equality, makeErr := state.manager.MkEq(leftWord, right[0])
				if makeErr != nil {
					return false, makeErr
				}
				constraint, makeErr := state.manager.MkImplies(equality, choices)
				if makeErr != nil {
					return false, makeErr
				}
				if _, found := t.axioms.set[constraint]; found {
					return false, nil
				}
				return true, t.axioms.Push(constraint)
			}
		}
		left, right = right, left
	}
	return false, nil
}
