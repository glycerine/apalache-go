package smtcore

// Source: src/smt/theory_str.cpp process_concat_eq_unroll.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) processStringConcatUnroll(concat, unroll ast.ASTHandle) (result error) {
	key := [2]ast.ASTHandle{concat, unroll}
	if cached, found := t.testers.concatUnrollDefinitions[key]; found {
		return t.assertAxiom(cached)
	}
	m := t.state.manager
	left, right, ok, err := m.SequenceConcatParts(concat)
	if err != nil {
		return err
	}
	if !ok {
		return ast.ErrASTDomain
	}
	op, own, err := t.operation(unroll)
	if err != nil {
		return err
	}
	if !own || op != 43 {
		return ast.ErrASTDomain
	}
	regex, err := m.GetArg(unroll, 0)
	if err != nil {
		return err
	}
	count, err := m.GetArg(unroll, 1)
	if err != nil {
		return err
	}
	locals := smtStringLengthPremises{theory: t}
	zeroItems := smtStringLengthPremises{theory: t}
	positiveItems := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, positiveItems.close(), zeroItems.close(), locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	lengthOf := func(term ast.ASTHandle) (ast.ASTHandle, error) { return pin(m.MkSequenceLength(term)) }
	appendEq := func(items *smtStringLengthPremises, left, right ast.ASTHandle) error {
		equal, err := t.state.mkEqualityAtom(left, right)
		if err != nil {
			return err
		}
		return items.add(equal)
	}
	var parts [5]ast.ASTHandle
	for i := range parts {
		parts[i], err = t.freshRegexVariable()
		if err != nil {
			return err
		}
	}
	firstCount, err := t.freshUnrollBound()
	if err != nil {
		return err
	}
	secondCount, err := t.freshUnrollBound()
	if err != nil {
		return err
	}
	empty, err := pin(t.stringLiteral(""))
	if err != nil {
		return err
	}
	firstUnroll, err := pin(t.makeStringUnroll(regex, firstCount))
	if err != nil {
		return err
	}
	secondUnroll, err := pin(t.makeStringUnroll(regex, secondCount))
	if err != nil {
		return err
	}
	zero, err := pin(m.ArithmeticIntegerNumeralText("0"))
	if err != nil {
		return err
	}
	zeroCase, err := pin(t.state.mkEqualityAtom(count, zero))
	if err != nil {
		return err
	}
	one, err := pin(m.ArithmeticIntegerNumeralText("1"))
	if err != nil {
		return err
	}
	positiveCase, err := pin(m.MkArithmeticGE(count, one))
	if err != nil {
		return err
	}
	if err = appendEq(&zeroItems, left, empty); err != nil {
		return err
	}
	if err = appendEq(&zeroItems, right, empty); err != nil {
		return err
	}
	leftLength, err := lengthOf(left)
	if err != nil {
		return err
	}
	rightLength, err := lengthOf(right)
	if err != nil {
		return err
	}
	if err = appendEq(&zeroItems, leftLength, zero); err != nil {
		return err
	}
	if err = appendEq(&zeroItems, rightLength, zero); err != nil {
		return err
	}
	zeroBody, err := pin(m.MkAndArray(zeroItems.values))
	if err != nil {
		return err
	}
	zeroEquivalent, err := pin(t.state.mkEqualityAtom(zeroCase, zeroBody))
	if err != nil {
		return err
	}
	for _, entry := range []struct{ whole, first, second, length ast.ASTHandle }{{left, parts[0], parts[1], leftLength}, {right, parts[2], parts[3], rightLength}} {
		joined, err := pin(t.concatTerm(entry.first, entry.second))
		if err != nil {
			return err
		}
		if err = appendEq(&positiveItems, entry.whole, joined); err != nil {
			return err
		}
		l, err := lengthOf(entry.first)
		if err != nil {
			return err
		}
		r, err := lengthOf(entry.second)
		if err != nil {
			return err
		}
		sum, err := pin(m.MkArithmeticAdd(l, r))
		if err != nil {
			return err
		}
		if err = appendEq(&positiveItems, entry.length, sum); err != nil {
			return err
		}
	}
	for _, entry := range [][2]ast.ASTHandle{{parts[0], firstUnroll}, {parts[3], secondUnroll}} {
		if err = appendEq(&positiveItems, entry[0], entry[1]); err != nil {
			return err
		}
		l, err := lengthOf(entry[0])
		if err != nil {
			return err
		}
		r, err := lengthOf(entry[1])
		if err != nil {
			return err
		}
		if err = appendEq(&positiveItems, l, r); err != nil {
			return err
		}
	}
	boundary, err := pin(t.concatTerm(parts[1], parts[2]))
	if err != nil {
		return err
	}
	if err = appendEq(&positiveItems, parts[4], boundary); err != nil {
		return err
	}
	if err = t.reduceStringVirtualRegexIn(parts[4], regex, &positiveItems); err != nil {
		return err
	}
	boundaryLength, err := lengthOf(parts[4])
	if err != nil {
		return err
	}
	firstLength, err := lengthOf(parts[1])
	if err != nil {
		return err
	}
	secondLength, err := lengthOf(parts[2])
	if err != nil {
		return err
	}
	boundarySum, err := pin(m.MkArithmeticAdd(firstLength, secondLength))
	if err != nil {
		return err
	}
	if err = appendEq(&positiveItems, boundaryLength, boundarySum); err != nil {
		return err
	}
	countSum, err := pin(m.MkArithmeticAdd(firstCount, secondCount))
	if err != nil {
		return err
	}
	minusOne, err := pin(m.ArithmeticIntegerNumeralText("-1"))
	if err != nil {
		return err
	}
	predecessor, err := pin(m.MkArithmeticAdd(count, minusOne))
	if err != nil {
		return err
	}
	if err = appendEq(&positiveItems, countSum, predecessor); err != nil {
		return err
	}
	positiveBody, err := pin(m.MkAndArray(positiveItems.values))
	if err != nil {
		return err
	}
	positiveEquivalent, err := pin(t.state.mkEqualityAtom(positiveCase, positiveBody))
	if err != nil {
		return err
	}
	formula, err := pin(m.MkAnd(zeroEquivalent, positiveEquivalent))
	if err != nil {
		return err
	}
	if err = t.retain(formula); err != nil {
		return err
	}
	t.testers.concatUnrollDefinitions[key] = formula
	return t.assertAxiom(formula)
}
