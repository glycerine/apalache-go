package smtcore

// Source: src/smt/theory_seq.cpp add_itos_axiom and add_stoi_axiom retained
// arithmetic, empty-string and round-trip constraints.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTSequenceTheory) addIntToStringAxioms(expression ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	integer, matched, err := state.manager.SequenceIntToStringArgument(expression)
	if err != nil || !matched {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	sort, err := state.manager.GetSort(expression)
	if err != nil {
		return err
	}
	empty, err := state.manager.MkSequenceEmpty(sort)
	if err != nil {
		return err
	}
	emptyEquality, err := state.manager.MkEq(expression, empty)
	if err != nil {
		return err
	}
	zero, err := t.integer(0)
	if err != nil {
		return err
	}
	nonnegative, err := state.manager.MkArithmeticGE(integer, zero)
	if err != nil {
		return err
	}
	notNonnegative, err := state.manager.MkNot(nonnegative)
	if err != nil {
		return err
	}
	emptyOnlyNegative, err := state.manager.MkImplies(emptyEquality, notNonnegative)
	if err != nil {
		return err
	}
	if err = t.axioms.Push(emptyOnlyNegative); err != nil {
		return err
	}
	notEmpty, err := state.manager.MkNot(emptyEquality)
	if err != nil {
		return err
	}
	nonemptyOnlyNonnegative, err := state.manager.MkImplies(notEmpty, nonnegative)
	if err != nil {
		return err
	}
	if err = t.axioms.Push(nonemptyOnlyNonnegative); err != nil {
		return err
	}
	back, err := state.manager.MkStringToInt(expression)
	if err != nil {
		return err
	}
	roundTrip, err := state.manager.MkEq(back, integer)
	if err != nil {
		return err
	}
	guardedRoundTrip, err := state.manager.MkImplies(nonnegative, roundTrip)
	if err != nil {
		return err
	}
	if err = t.axioms.Push(guardedRoundTrip); err != nil {
		return err
	}
	numeric, err := t.numericStringMembership(expression)
	if err != nil {
		return err
	}
	digitsOnlyWhenNonnegative, err := state.manager.MkImplies(nonnegative, numeric)
	if err != nil {
		return err
	}
	if err = t.axioms.Push(digitsOnlyWhenNonnegative); err != nil {
		return err
	}
	return t.addConcreteIntToStringAxiom(expression, integer, back)
}

func (t *SMTSequenceTheory) addStringToIntAxioms(expression ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	sequence, matched, err := state.manager.SequenceUnaryOperationArgument(expression, 28)
	if err != nil || !matched {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	minusOne, err := state.manager.ArithmeticIntegerNumeralText("-1")
	if err != nil {
		return err
	}
	lowerBound, err := state.manager.MkArithmeticGE(expression, minusOne)
	if err != nil {
		return err
	}
	if err = t.axioms.Push(lowerBound); err != nil {
		return err
	}
	zero, err := t.integer(0)
	if err != nil {
		return err
	}
	nonnegative, err := state.manager.MkArithmeticGE(expression, zero)
	if err != nil {
		return err
	}
	numeric, err := t.numericStringMembership(sequence)
	if err != nil {
		return err
	}
	equivalence, err := state.manager.MkIff(nonnegative, numeric)
	if err != nil {
		return err
	}
	if err = t.axioms.Push(equivalence); err != nil {
		return err
	}
	return t.addConcreteStringToIntAxiom(expression, sequence)
}

func (t *SMTSequenceTheory) numericStringMembership(sequence ast.ASTHandle) (ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zero, err := state.manager.MkString("0")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	nine, err := state.manager.MkString("9")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	digit, err := state.manager.MkRegexRange(zero, nine)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	digits, err := state.manager.MkRegexPlus(digit)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return state.manager.MkSequenceInRegex(sequence, digits)
}

func (t *SMTSequenceTheory) addConcreteIntToStringAxiom(expression, integer, roundTrip ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	text, integral, matched, err := state.manager.ArithmeticNumeralText(integer)
	if err != nil || !matched || !integral {
		return err
	}
	value, ok := new(big.Int).SetString(text, 10)
	if !ok {
		return ast.ErrASTDomain
	}
	result := ""
	if value.Sign() >= 0 {
		result = value.String()
	}
	stringValue, err := state.manager.MkString(result)
	if err != nil {
		return err
	}
	equality, err := state.manager.MkEq(expression, stringValue)
	if err != nil {
		return err
	}
	t.trackInternalTerms(expression, roundTrip)
	return t.axioms.Push(equality)
}

func (t *SMTSequenceTheory) addConcreteStringToIntAxiom(expression, sequence ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	text, matched, err := state.manager.SequenceStringValue(sequence)
	if err != nil || !matched {
		return err
	}
	value := new(big.Int).SetInt64(-1)
	if text != "" {
		digits := true
		for index := 0; index < len(text); index++ {
			if text[index] < '0' || text[index] > '9' {
				digits = false
				break
			}
		}
		if digits {
			if _, ok := value.SetString(text, 10); !ok {
				return ast.ErrASTDomain
			}
		}
	}
	numeral, err := state.manager.ArithmeticIntegerNumeralText(value.String())
	if err != nil {
		return err
	}
	equality, err := state.manager.MkEq(expression, numeral)
	if err != nil {
		return err
	}
	t.trackInternalTerms(expression)
	return t.axioms.Push(equality)
}
