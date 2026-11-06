package smtcore

// Source: src/smt/theory_seq.cpp fixed_length and mk_decompose.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

const smtSequenceMaximumFixedDecomposition = 2048

func smtSequenceIntegerLowerBound(value *big.Rat, epsilonSign int) (uint64, bool) {
	if value == nil || value.Sign() < 0 {
		return 0, false
	}
	quotient := new(big.Int)
	remainder := new(big.Int)
	quotient.QuoRem(value.Num(), value.Denom(), remainder)
	if remainder.Sign() != 0 || epsilonSign > 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return quotient.Uint64(), quotient.IsUint64()
}

func smtSequenceIntegerUpperBound(value *big.Rat, open bool) (uint64, bool) {
	if value == nil || value.Sign() < 0 {
		return 0, false
	}
	quotient := new(big.Int)
	remainder := new(big.Int)
	quotient.QuoRem(value.Num(), value.Denom(), remainder)
	if open && remainder.Sign() == 0 {
		quotient.Sub(quotient, big.NewInt(1))
	}
	return quotient.Uint64(), quotient.Sign() >= 0 && quotient.IsUint64()
}

func (t *SMTSequenceTheory) addLengthLowerBoundDecomposition() (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	arithmeticFamily, err := state.manager.ArithmeticFamilyID()
	if err != nil {
		return false, err
	}
	arithmetic, ok := state.Theory(arithmeticFamily).(*SMTLinearArithmeticTheory)
	if !ok {
		return false, nil
	}
	intervals, err := arithmetic.linearVariableIntervals()
	if err != nil {
		return false, err
	}
	seen := make(map[ast.ASTHandle]struct{})
	for _, node := range t.variables {
		if node == nil {
			continue
		}
		expression := node.Owner()
		if _, tracked := t.lengths[expression]; !tracked {
			continue
		}
		if _, found := seen[expression]; found {
			continue
		}
		seen[expression] = struct{}{}
		variable, variableErr := t.sequenceVariable(expression)
		if variableErr != nil {
			return false, variableErr
		}
		if !variable || !t.solutions.IsRoot(expression) {
			continue
		}
		length, makeErr := state.manager.MkSequenceLength(expression)
		if makeErr != nil {
			return false, makeErr
		}
		interval, evaluated, intervalErr := arithmetic.polynomialInterval(length, intervals)
		if intervalErr != nil {
			return false, intervalErr
		}
		if !evaluated || interval.minusInfinity() {
			continue
		}
		lower, lowerErr := interval.lowerValue()
		if lowerErr != nil {
			return false, lowerErr
		}
		epsilonSign := 0
		if interval.lowerOpen {
			epsilonSign = 1
		}
		count, integral := smtSequenceIntegerLowerBound(lower, epsilonSign)
		if !integral || count == 0 || count >= smtSequenceMaximumFixedDecomposition {
			continue
		}
		sort, makeErr := state.manager.GetSort(expression)
		if makeErr != nil {
			return false, makeErr
		}
		elementSort, sequence, makeErr := state.manager.SequenceSortElement(sort)
		if makeErr != nil || !sequence {
			if makeErr != nil {
				return false, makeErr
			}
			return false, ast.ErrASTDomain
		}
		parts := make([]ast.ASTHandle, 0, int(count)+1)
		internal := make([]ast.ASTHandle, 0, int(count)*2+2)
		var lastPosition ast.ASTHandle
		for index := uint64(0); index < count; index++ {
			position, positionErr := t.integer(index)
			if positionErr != nil {
				return false, positionErr
			}
			lastPosition = position
			element, positionErr := t.relationSkolem("seq.nth", []ast.ASTHandle{expression, position}, elementSort)
			if positionErr != nil {
				return false, positionErr
			}
			unit, positionErr := state.manager.MkSequenceUnit(element)
			if positionErr != nil {
				return false, positionErr
			}
			parts = append(parts, unit)
			internal = append(internal, element, unit)
		}
		remainder, makeErr := t.relationSkolem("seq.tail", []ast.ASTHandle{expression, lastPosition}, sort)
		if makeErr != nil {
			return false, makeErr
		}
		parts = append(parts, remainder)
		word, makeErr := t.makeWord(parts, sort)
		if makeErr != nil {
			return false, makeErr
		}
		boundValue, makeErr := t.integer(count)
		if makeErr != nil {
			return false, makeErr
		}
		bound, makeErr := state.manager.MkArithmeticGE(length, boundValue)
		if makeErr != nil {
			return false, makeErr
		}
		definition, makeErr := state.manager.MkEq(expression, word)
		if makeErr != nil {
			return false, makeErr
		}
		constraint, makeErr := state.manager.MkImplies(bound, definition)
		if makeErr != nil {
			return false, makeErr
		}
		progress := false
		if _, found := t.axioms.set[constraint]; !found {
			internal = append(internal, remainder, word)
			t.trackInternalTerms(internal...)
			if makeErr = t.axioms.Push(constraint); makeErr != nil {
				return false, makeErr
			}
			progress = true
		}
		if !interval.plusInfinity() {
			upper, upperErr := interval.upperValue()
			if upperErr != nil {
				return false, upperErr
			}
			upperCount, upperIntegral := smtSequenceIntegerUpperBound(upper, interval.upperOpen)
			if upperIntegral && upperCount >= count {
				upperValue, boundErr := t.integer(upperCount)
				if boundErr != nil {
					return false, boundErr
				}
				upperBound, boundErr := state.manager.MkArithmeticLE(length, upperValue)
				if boundErr != nil {
					return false, boundErr
				}
				remainderLength, boundErr := state.manager.MkSequenceLength(remainder)
				if boundErr != nil {
					return false, boundErr
				}
				remainderMaximum, boundErr := t.integer(upperCount - count)
				if boundErr != nil {
					return false, boundErr
				}
				remainderBound, boundErr := state.manager.MkArithmeticLE(remainderLength, remainderMaximum)
				if boundErr != nil {
					return false, boundErr
				}
				upperConstraint, boundErr := state.manager.MkImplies(upperBound, remainderBound)
				if boundErr != nil {
					return false, boundErr
				}
				if _, found := t.axioms.set[upperConstraint]; !found {
					t.trackInternalTerms(remainder)
					if boundErr = t.axioms.Push(upperConstraint); boundErr != nil {
						return false, boundErr
					}
					progress = true
				}
			}
		} else {
			empty, branchErr := state.manager.MkSequenceEmpty(sort)
			if branchErr != nil {
				return false, branchErr
			}
			assignment, branchErr := t.assumeSequenceEquality(remainder, empty)
			if branchErr != nil {
				return false, branchErr
			}
			if assignment == lbool.Undef {
				return true, nil
			}
		}
		if progress {
			return true, nil
		}
	}
	return false, nil
}

func (t *SMTSequenceTheory) addFixedLengthDecomposition() (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	arithmeticFamily, err := state.manager.ArithmeticFamilyID()
	if err != nil {
		return false, err
	}
	arithmetic, ok := state.Theory(arithmeticFamily).(*SMTLinearArithmeticTheory)
	if !ok {
		return false, nil
	}
	fixed := arithmetic.fixedLinearAssignments()
	seen := make(map[ast.ASTHandle]struct{})
	for _, node := range t.variables {
		if node == nil {
			continue
		}
		expression := node.Owner()
		if _, tracked := t.lengths[expression]; !tracked {
			continue
		}
		if _, found := seen[expression]; found {
			continue
		}
		seen[expression] = struct{}{}
		variable, variableErr := t.sequenceVariable(expression)
		if variableErr != nil {
			return false, variableErr
		}
		if !variable {
			continue
		}
		op, own, opErr := t.operation(expression)
		if opErr != nil {
			return false, opErr
		}
		if own && op == 42 {
			decl, nameErr := state.manager.GetDecl(expression)
			if nameErr != nil {
				return false, nameErr
			}
			name, nameErr := state.manager.GetASTName(decl)
			if nameErr != nil {
				return false, nameErr
			}
			text, nameErr := state.manager.Symbols.String(name)
			if nameErr != nil {
				return false, nameErr
			}
			switch text {
			case "seq.tail", "seq.first", "seq.idx.left", "seq.idx.right":
				continue
			}
		}
		length, makeErr := state.manager.MkSequenceLength(expression)
		if makeErr != nil {
			return false, makeErr
		}
		value, _, exact, valueErr := arithmetic.fixedPolynomialValue(length, fixed)
		if valueErr != nil {
			return false, valueErr
		}
		if !exact {
			lengthNode := state.context.enodes[length]
			if lengthNode != nil {
				text, integer, numeral, numeralErr := state.manager.ArithmeticNumeralText(lengthNode.Root().Owner())
				if numeralErr != nil {
					return false, numeralErr
				}
				if integer && numeral {
					if parsed, parsedOK := new(big.Rat).SetString(text); parsedOK {
						value, exact = parsed, true
					}
				}
			}
		}
		if !exact || !value.IsInt() || value.Sign() < 0 || !value.Num().IsUint64() {
			continue
		}
		count := value.Num().Uint64()
		if count > smtSequenceMaximumFixedDecomposition {
			continue
		}
		sort, makeErr := state.manager.GetSort(expression)
		if makeErr != nil {
			return false, makeErr
		}
		elementSort, sequence, makeErr := state.manager.SequenceSortElement(sort)
		if makeErr != nil || !sequence {
			if makeErr != nil {
				return false, makeErr
			}
			return false, ast.ErrASTDomain
		}
		parts := make([]ast.ASTHandle, 0, int(count))
		internal := make([]ast.ASTHandle, 0, int(count)*2)
		for index := uint64(0); index < count; index++ {
			position, positionErr := t.integer(index)
			if positionErr != nil {
				return false, positionErr
			}
			element, positionErr := t.relationSkolem("seq.nth", []ast.ASTHandle{expression, position}, elementSort)
			if positionErr != nil {
				return false, positionErr
			}
			unit, positionErr := state.manager.MkSequenceUnit(element)
			if positionErr != nil {
				return false, positionErr
			}
			parts = append(parts, unit)
			internal = append(internal, element, unit)
		}
		word, makeErr := t.makeWord(parts, sort)
		if makeErr != nil {
			return false, makeErr
		}
		lengthValue, makeErr := t.integer(count)
		if makeErr != nil {
			return false, makeErr
		}
		fixedLength, makeErr := state.manager.MkEq(length, lengthValue)
		if makeErr != nil {
			return false, makeErr
		}
		definition, makeErr := state.manager.MkEq(expression, word)
		if makeErr != nil {
			return false, makeErr
		}
		constraint, makeErr := state.manager.MkImplies(fixedLength, definition)
		if makeErr != nil {
			return false, makeErr
		}
		if _, found := t.axioms.set[constraint]; found {
			continue
		}
		internal = append(internal, word)
		t.trackInternalTerms(internal...)
		return true, t.axioms.Push(constraint)
	}
	return false, nil
}
