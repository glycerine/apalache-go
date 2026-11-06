package smtcore

// Source: src/smt/theory_seq.cpp add_length_axiom and the retained exact
// concat, unit, empty, string, int.to.str and non-negativity cases.

import (
	"math/big"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

// Source relevant_eh/add_length/enforce_length: a relevant length term tracks
// every sequence expression in its equivalence class, with scoped ownership.
func (t *SMTSequenceTheory) Relevant(expression ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	argument, matched, err := state.manager.SequenceUnaryOperationArgument(expression, 9)
	if err != nil || !matched {
		return err
	}
	if _, tracked := t.lengths[argument]; tracked {
		return nil
	}
	if node := state.context.enodes[argument]; node != nil {
		return t.enforceSequenceLength(node)
	}
	return nil
}

func (t *SMTSequenceTheory) enforceSequenceLength(node *SMTEnode) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	if node == nil || node.manager != state.manager {
		return ast.ErrASTDomain
	}
	for current := node; ; current = current.Next() {
		expression := current.Owner()
		if _, tracked := t.lengths[expression]; !tracked {
			length, err := state.manager.MkSequenceLength(expression)
			if err != nil {
				return err
			}
			ref, err := state.manager.NewRef(expression)
			if err != nil {
				return err
			}
			t.lengths[expression] = struct{}{}
			t.lengthLog = append(t.lengthLog, ref)
			state.theoryProgress++
			// The Go axiom queue contains the Boolean consequences rather
			// than source non-Boolean axiom requests. Internalizing length
			// installs its existing concrete length axioms into that queue.
			if err = state.Internalize(length, false); err != nil {
				return err
			}
		}
		if current.Next() == node {
			break
		}
	}
	return nil
}

func (t *SMTSequenceTheory) enforceSequenceLengthCoherence(first, second *SMTEnode) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	leftConcat, err := state.manager.IsSequenceConcat(first.Owner())
	if err != nil {
		return err
	}
	rightConcat, err := state.manager.IsSequenceConcat(second.Owner())
	if err != nil {
		return err
	}
	if leftConcat && rightConcat {
		return nil
	}
	_, left := t.lengths[first.Owner()]
	_, right := t.lengths[second.Owner()]
	if left && !right {
		return t.enforceSequenceLength(second)
	}
	if right && !left {
		return t.enforceSequenceLength(first)
	}
	return nil
}

func (t *SMTSequenceTheory) addIntegerStringLengthRefinement() (bool, error) {
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
		conversion := node.Owner()
		if _, found := seen[conversion]; found {
			continue
		}
		seen[conversion] = struct{}{}
		integer, matched, matchErr := state.manager.SequenceIntToStringArgument(conversion)
		if matchErr != nil {
			return false, matchErr
		}
		if !matched {
			continue
		}
		length, makeErr := state.manager.MkSequenceLength(conversion)
		if makeErr != nil {
			return false, makeErr
		}
		width := uint64(1)
		integerValue, _, integerFixed, valueErr := arithmetic.fixedPolynomialValue(integer, fixed)
		if valueErr != nil {
			return false, valueErr
		}
		if integerFixed && integerValue.IsInt() && integerValue.Sign() >= 0 {
			width = uint64(len(integerValue.Num().String()))
		}
		lengthValue, _, lengthFixed, valueErr := arithmetic.fixedPolynomialValue(length, fixed)
		if valueErr != nil {
			return false, valueErr
		}
		if lengthFixed && lengthValue.IsInt() && lengthValue.Sign() > 0 && lengthValue.Num().IsUint64() && lengthValue.Num().Uint64() > width {
			width = lengthValue.Num().Uint64()
		}
		if width <= 1 || width >= smtSequenceMaximumFixedDecomposition {
			continue
		}
		lowerPower := new(big.Int).Exp(big.NewInt(10), new(big.Int).SetUint64(width-1), nil)
		upperPower := new(big.Int).Mul(new(big.Int).Set(lowerPower), big.NewInt(10))
		lower, makeErr := state.manager.ArithmeticIntegerNumeralText(lowerPower.String())
		if makeErr != nil {
			return false, makeErr
		}
		upper, makeErr := state.manager.ArithmeticIntegerNumeralText(upperPower.String())
		if makeErr != nil {
			return false, makeErr
		}
		widthValue, makeErr := t.integer(width)
		if makeErr != nil {
			return false, makeErr
		}
		integerAtLeastLower, makeErr := state.manager.MkArithmeticGE(integer, lower)
		if makeErr != nil {
			return false, makeErr
		}
		lengthAtLeastWidth, makeErr := state.manager.MkArithmeticGE(length, widthValue)
		if makeErr != nil {
			return false, makeErr
		}
		lowerConstraint, makeErr := state.manager.MkImplies(integerAtLeastLower, lengthAtLeastWidth)
		if makeErr != nil {
			return false, makeErr
		}
		integerAtLeastUpper, makeErr := state.manager.MkArithmeticGE(integer, upper)
		if makeErr != nil {
			return false, makeErr
		}
		lengthAtMostWidth, makeErr := state.manager.MkArithmeticLE(length, widthValue)
		if makeErr != nil {
			return false, makeErr
		}
		lengthAboveWidth, makeErr := state.manager.MkNot(lengthAtMostWidth)
		if makeErr != nil {
			return false, makeErr
		}
		upperConstraint, makeErr := state.manager.MkImplies(integerAtLeastUpper, lengthAboveWidth)
		if makeErr != nil {
			return false, makeErr
		}
		progress := false
		for _, constraint := range []ast.ASTHandle{lowerConstraint, upperConstraint} {
			if _, found := t.axioms.set[constraint]; found {
				continue
			}
			if makeErr = t.axioms.Push(constraint); makeErr != nil {
				return false, makeErr
			}
			progress = true
		}
		if progress {
			return true, nil
		}
	}
	return false, nil
}

func (t *SMTSequenceTheory) assertConstraint(expression ast.ASTHandle) error {
	state, err := t.State()
	if err != nil || state.manager.IsTrue(expression) {
		return err
	}
	if err = state.Internalize(expression, false); err != nil {
		return err
	}
	literal, err := state.Literal(expression)
	if err != nil {
		return err
	}
	return state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{literal})
}

func (t *SMTSequenceTheory) integer(value uint64) (ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return state.manager.ArithmeticIntegerNumeralText(strconv.FormatUint(value, 10))
}

func (t *SMTSequenceTheory) lengthDefinition(argument ast.ASTHandle) (ast.ASTHandle, bool, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if text, matched, textErr := state.manager.SequenceStringValue(argument); textErr != nil || matched {
		if textErr != nil {
			return ast.ASTHandle{}, false, textErr
		}
		value, makeErr := t.integer(uint64(len(text)))
		return value, true, makeErr
	}
	if empty, emptyErr := state.manager.IsSequenceEmpty(argument); emptyErr != nil || empty {
		if emptyErr != nil {
			return ast.ASTHandle{}, false, emptyErr
		}
		value, makeErr := t.integer(0)
		return value, true, makeErr
	}
	if _, unit, unitErr := state.manager.SequenceUnitArgument(argument); unitErr != nil || unit {
		if unitErr != nil {
			return ast.ASTHandle{}, false, unitErr
		}
		value, makeErr := t.integer(1)
		return value, true, makeErr
	}
	left, right, concat, concatErr := state.manager.SequenceConcatParts(argument)
	if concatErr != nil || !concat {
		return ast.ASTHandle{}, false, concatErr
	}
	leftLength, err := state.manager.MkSequenceLength(left)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	rightLength, err := state.manager.MkSequenceLength(right)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	value, err := state.manager.MkArithmeticAdd(leftLength, rightLength)
	return value, true, err
}

func (t *SMTSequenceTheory) addLengthAxiom(length ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	argument, matched, err := state.manager.SequenceUnaryOperationArgument(length, 9)
	if err != nil || !matched {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	definition, defined, err := t.lengthDefinition(argument)
	if err != nil {
		return err
	}
	if defined {
		equality, makeErr := state.manager.MkEq(length, definition)
		if makeErr != nil {
			return makeErr
		}
		return t.axioms.Push(equality)
	}
	zero, err := t.integer(0)
	if err != nil {
		return err
	}
	nonnegative, err := state.manager.MkArithmeticGE(length, zero)
	if err != nil {
		return err
	}
	if err = t.axioms.Push(nonnegative); err != nil {
		return err
	}
	integer, itos, err := state.manager.SequenceIntToStringArgument(argument)
	if err != nil || !itos {
		return err
	}
	one, err := t.integer(1)
	if err != nil {
		return err
	}
	integerNonnegative, err := state.manager.MkArithmeticGE(integer, zero)
	if err != nil {
		return err
	}
	lengthAtLeastOne, err := state.manager.MkArithmeticGE(length, one)
	if err != nil {
		return err
	}
	implication, err := state.manager.MkImplies(integerNonnegative, lengthAtLeastOne)
	if err != nil {
		return err
	}
	if err = t.axioms.Push(implication); err != nil {
		return err
	}
	ten, err := t.integer(10)
	if err != nil {
		return err
	}
	integerAtLeastTen, err := state.manager.MkArithmeticGE(integer, ten)
	if err != nil {
		return err
	}
	integerBelowTen, err := state.manager.MkNot(integerAtLeastTen)
	if err != nil {
		return err
	}
	lengthAtMostOne, err := state.manager.MkArithmeticLE(length, one)
	if err != nil {
		return err
	}
	if err = t.pushSequenceImplication([]ast.ASTHandle{integerNonnegative, integerBelowTen}, lengthAtMostOne); err != nil {
		return err
	}
	lengthAboveOne, err := state.manager.MkNot(lengthAtMostOne)
	if err != nil {
		return err
	}
	return t.pushSequenceImplication([]ast.ASTHandle{integerAtLeastTen}, lengthAboveOne)
}
