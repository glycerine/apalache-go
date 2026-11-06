package smtcore

// Source: src/smt/theory_seq.cpp reduce_length_eq and reduce_length.

import (
	"math"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTSequenceTheory) exactSequenceLength(expression ast.ASTHandle) (uint64, bool, error) {
	state, err := t.State()
	if err != nil {
		return 0, false, err
	}
	if text, matched, textErr := state.manager.SequenceStringValue(expression); textErr != nil || matched {
		return uint64(len(text)), matched, textErr
	}
	if empty, emptyErr := state.manager.IsSequenceEmpty(expression); emptyErr != nil || empty {
		return 0, empty, emptyErr
	}
	if _, unit, unitErr := state.manager.SequenceUnitArgument(expression); unitErr != nil || unit {
		return 1, unit, unitErr
	}
	length, err := state.manager.MkSequenceLength(expression)
	if err != nil {
		return 0, false, err
	}
	arithmeticFamily, err := state.manager.ArithmeticFamilyID()
	if err != nil {
		return 0, false, err
	}
	arithmetic, ok := state.Theory(arithmeticFamily).(*SMTLinearArithmeticTheory)
	if !ok {
		return 0, false, nil
	}
	value, _, exact, err := arithmetic.fixedPolynomialValue(length, arithmetic.fixedLinearAssignments())
	if err != nil {
		return 0, false, err
	}
	if !exact {
		if node := state.context.enodes[length]; node != nil {
			text, integer, numeral, numeralErr := state.manager.ArithmeticNumeralText(node.Root().Owner())
			if numeralErr != nil {
				return 0, false, numeralErr
			}
			if integer && numeral {
				value, exact = new(big.Rat).SetString(text)
			}
		}
	}
	if !exact || value == nil || !value.IsInt() || value.Sign() < 0 || !value.Num().IsUint64() {
		return 0, false, nil
	}
	return value.Num().Uint64(), true, nil
}

func (t *SMTSequenceTheory) fixedLengthPartition(parts []ast.ASTHandle, front bool) ([]uint64, bool, error) {
	// The final part remains on the residual side of every proper split, so its
	// length need not be fixed.
	lengths := make([]uint64, len(parts)-1)
	for index := range lengths {
		partIndex := index
		if !front {
			partIndex = len(parts) - 1 - index
		}
		length, exact, err := t.exactSequenceLength(parts[partIndex])
		if err != nil || !exact {
			return nil, exact, err
		}
		lengths[index] = length
	}
	return lengths, true, nil
}

func smtSequenceEqualPartition(left, right []uint64) (int, int, bool) {
	var leftSum, rightSum uint64
	for leftCount, rightCount := 0, 0; leftCount < len(left) || rightCount < len(right); {
		if leftSum <= rightSum && leftCount < len(left) {
			if left[leftCount] > math.MaxUint64-leftSum {
				return 0, 0, false
			}
			leftSum += left[leftCount]
			leftCount++
		} else if rightCount < len(right) {
			if right[rightCount] > math.MaxUint64-rightSum {
				return 0, 0, false
			}
			rightSum += right[rightCount]
			rightCount++
		} else {
			break
		}
		if leftSum == rightSum && leftSum != 0 && leftCount != 0 && rightCount != 0 {
			return leftCount, rightCount, true
		}
	}
	return 0, 0, false
}

func (t *SMTSequenceTheory) splitFixedLengthEquation(left, right []ast.ASTHandle) (bool, error) {
	if len(left) < 2 || len(right) < 2 {
		return false, nil
	}
	state, err := t.State()
	if err != nil {
		return false, err
	}
	sort, err := state.manager.GetSort(left[0])
	if err != nil {
		return false, err
	}
	for _, front := range []bool{true, false} {
		leftLengths, exact, lengthErr := t.fixedLengthPartition(left, front)
		if lengthErr != nil {
			return false, lengthErr
		}
		if !exact {
			continue
		}
		rightLengths, exact, lengthErr := t.fixedLengthPartition(right, front)
		if lengthErr != nil {
			return false, lengthErr
		}
		if !exact {
			continue
		}
		leftCount, rightCount, found := smtSequenceEqualPartition(leftLengths, rightLengths)
		if !found {
			continue
		}
		leftFirst, leftRest := left[:leftCount], left[leftCount:]
		rightFirst, rightRest := right[:rightCount], right[rightCount:]
		if !front {
			leftFirst, leftRest = left[len(left)-leftCount:], left[:len(left)-leftCount]
			rightFirst, rightRest = right[len(right)-rightCount:], right[:len(right)-rightCount]
		}
		leftPrefix, makeErr := t.makeWord(leftFirst, sort)
		if makeErr != nil {
			return false, makeErr
		}
		rightPrefix, makeErr := t.makeWord(rightFirst, sort)
		if makeErr != nil {
			return false, makeErr
		}
		leftRemainder, makeErr := t.makeWord(leftRest, sort)
		if makeErr != nil {
			return false, makeErr
		}
		rightRemainder, makeErr := t.makeWord(rightRest, sort)
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
		wordEquality, makeErr := state.manager.MkEq(leftWord, rightWord)
		if makeErr != nil {
			return false, makeErr
		}
		leftLength, makeErr := state.manager.MkSequenceLength(leftPrefix)
		if makeErr != nil {
			return false, makeErr
		}
		rightLength, makeErr := state.manager.MkSequenceLength(rightPrefix)
		if makeErr != nil {
			return false, makeErr
		}
		lengthEquality, makeErr := state.manager.MkEq(leftLength, rightLength)
		if makeErr != nil {
			return false, makeErr
		}
		antecedent, makeErr := state.manager.MkAnd(wordEquality, lengthEquality)
		if makeErr != nil {
			return false, makeErr
		}
		prefixEquality, makeErr := state.manager.MkEq(leftPrefix, rightPrefix)
		if makeErr != nil {
			return false, makeErr
		}
		remainderEquality, makeErr := state.manager.MkEq(leftRemainder, rightRemainder)
		if makeErr != nil {
			return false, makeErr
		}
		consequence, makeErr := state.manager.MkAnd(prefixEquality, remainderEquality)
		if makeErr != nil {
			return false, makeErr
		}
		constraint, makeErr := state.manager.MkImplies(antecedent, consequence)
		if makeErr != nil {
			return false, makeErr
		}
		if _, exists := t.axioms.set[constraint]; exists {
			continue
		}
		return true, t.axioms.Push(constraint)
	}
	return false, nil
}

func (t *SMTSequenceTheory) solveZeroLengthBoundary(left, right []ast.ASTHandle, dependency *smtSequenceDependency) (bool, error) {
	for pass := 0; pass < 2; pass++ {
		if len(left) != 0 {
			variable, err := t.sequenceVariable(left[0])
			if err != nil {
				return false, err
			}
			length, exact, err := t.exactSequenceLength(left[0])
			if err != nil {
				return false, err
			}
			if variable && exact && length == 0 {
				state, stateErr := t.State()
				if stateErr != nil {
					return false, stateErr
				}
				sort, sortErr := state.manager.GetSort(left[0])
				if sortErr != nil {
					return false, sortErr
				}
				empty, makeErr := state.manager.MkSequenceEmpty(sort)
				if makeErr != nil {
					return false, makeErr
				}
				return t.addSolution(left[0], empty, dependency)
			}
		}
		left, right = right, left
	}
	return false, nil
}

func (t *SMTSequenceTheory) rejectFixedLengthMismatch(left, right []ast.ASTHandle, dependency *smtSequenceDependency) (bool, error) {
	var totals [2]uint64
	for side, parts := range [][]ast.ASTHandle{left, right} {
		for _, part := range parts {
			length, exact, err := t.exactSequenceLength(part)
			if err != nil {
				return false, err
			}
			if !exact {
				return false, nil
			}
			if length > math.MaxUint64-totals[side] {
				return false, nil
			}
			totals[side] += length
		}
	}
	if totals[0] == totals[1] {
		return false, nil
	}
	return true, t.setDependencyConflict(dependency)
}

func (t *SMTSequenceTheory) branchLeadingVariableOverlap(left, right []ast.ASTHandle) (bool, error) {
	for pass := 0; pass < 2; pass++ {
		if len(left) > 1 && len(right) > 1 {
			leftVariable, err := t.sequenceVariable(left[0])
			if err != nil {
				return false, err
			}
			if !leftVariable {
				left, right = right, left
				continue
			}
			for rightIndex, candidate := range right {
				rightVariable, variableErr := t.sequenceVariable(candidate)
				if variableErr != nil {
					return false, variableErr
				}
				if !rightVariable || left[0] == candidate {
					continue
				}
				state, stateErr := t.State()
				if stateErr != nil {
					return false, stateErr
				}
				if rightIndex != 0 {
					fixedPrefix := true
					for _, part := range right[:rightIndex] {
						if _, fixedString, textErr := state.manager.SequenceStringValue(part); textErr != nil {
							return false, textErr
						} else if fixedString {
							continue
						}
						if _, unit, unitErr := state.manager.SequenceUnitArgument(part); unitErr != nil {
							return false, unitErr
						} else if !unit {
							fixedPrefix = false
							break
						}
					}
					if fixedPrefix {
						continue
					}
				}
				sort, makeErr := state.manager.GetSort(left[0])
				if makeErr != nil {
					return false, makeErr
				}
				prefix, makeErr := t.makeWord(right[:rightIndex], sort)
				if makeErr != nil {
					return false, makeErr
				}
				arguments := []ast.ASTHandle{left[0], prefix, candidate}
				shared, makeErr := t.relationSkolem("seq.left", arguments, sort)
				if makeErr != nil {
					return false, makeErr
				}
				remainder, makeErr := t.relationSkolem("seq.right", arguments, sort)
				if makeErr != nil {
					return false, makeErr
				}
				leftValue := shared
				if rightIndex != 0 {
					leftValue, makeErr = state.manager.MkSequenceConcat(prefix, shared)
					if makeErr != nil {
						return false, makeErr
					}
				}
				leftDefinition, makeErr := state.manager.MkEq(left[0], leftValue)
				if makeErr != nil {
					return false, makeErr
				}
				rightWord, makeErr := state.manager.MkSequenceConcat(shared, remainder)
				if makeErr != nil {
					return false, makeErr
				}
				rightDefinition, makeErr := state.manager.MkEq(candidate, rightWord)
				if makeErr != nil {
					return false, makeErr
				}
				definitions, makeErr := state.manager.MkAnd(leftDefinition, rightDefinition)
				if makeErr != nil {
					return false, makeErr
				}
				leftLength, makeErr := state.manager.MkSequenceLength(left[0])
				if makeErr != nil {
					return false, makeErr
				}
				rightLength, makeErr := state.manager.MkSequenceLength(candidate)
				if makeErr != nil {
					return false, makeErr
				}
				var abovePrefix, withinCandidate ast.ASTHandle
				if rightIndex == 0 {
					withinCandidate, makeErr = state.manager.MkArithmeticLE(leftLength, rightLength)
					if makeErr != nil {
						return false, makeErr
					}
				} else {
					prefixLength, lengthErr := state.manager.MkSequenceLength(prefix)
					if lengthErr != nil {
						return false, lengthErr
					}
					atMostPrefix, lengthErr := state.manager.MkArithmeticLE(leftLength, prefixLength)
					if lengthErr != nil {
						return false, lengthErr
					}
					abovePrefix, lengthErr = state.manager.MkNot(atMostPrefix)
					if lengthErr != nil {
						return false, lengthErr
					}
					prefixAndRightLength, lengthErr := state.manager.MkArithmeticAdd(prefixLength, rightLength)
					if lengthErr != nil {
						return false, lengthErr
					}
					withinCandidate, lengthErr = state.manager.MkArithmeticLE(leftLength, prefixAndRightLength)
					if lengthErr != nil {
						return false, lengthErr
					}
				}
				leftWord, makeErr := t.makeWord(left, sort)
				if makeErr != nil {
					return false, makeErr
				}
				fullRightWord, makeErr := t.makeWord(right, sort)
				if makeErr != nil {
					return false, makeErr
				}
				wordEquality, makeErr := state.manager.MkEq(leftWord, fullRightWord)
				if makeErr != nil {
					return false, makeErr
				}
				var antecedent ast.ASTHandle
				if rightIndex == 0 {
					antecedent, makeErr = state.manager.MkAnd(wordEquality, withinCandidate)
				} else {
					antecedent, makeErr = state.manager.MkAndArray([]ast.ASTHandle{wordEquality, abovePrefix, withinCandidate})
				}
				if makeErr != nil {
					return false, makeErr
				}
				constraint, makeErr := state.manager.MkImplies(antecedent, definitions)
				if makeErr != nil {
					return false, makeErr
				}
				if _, found := t.axioms.set[constraint]; found {
					if rightIndex == 0 {
						return false, nil
					}
					continue
				}
				t.trackInternalTerms(shared, remainder, leftValue, rightWord)
				return true, t.axioms.Push(constraint)
			}
		}
		left, right = right, left
	}
	return false, nil
}
