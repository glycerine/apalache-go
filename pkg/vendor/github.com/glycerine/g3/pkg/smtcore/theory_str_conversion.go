package smtcore

// Source: src/smt/theory_str.cpp instantiate_axiom_str_to_int and
// instantiate_axiom_int_to_str. These are the initial constraints; source
// finalcheck_str2int/finalcheck_int2str enforce the remaining value agreement.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) recordStringIntConversion(expression ast.ASTHandle) error {
	ref, err := t.state.manager.NewRef(expression)
	if err != nil {
		return err
	}
	// Native expr_ref_vector preserves repeated setup visits in insertion order.
	t.conversions = append(t.conversions, expression)
	t.conversionRefs = append(t.conversionRefs, ref)
	return nil
}

func (t *SMTStringTheory) instantiateStringToInt(node *SMTEnode) (result error) {
	m := t.state.manager
	expression := node.Owner()
	if _, found := t.axiomatized[expression]; found {
		return nil
	}
	t.axiomatized[expression] = struct{}{}
	word, err := m.GetArg(expression, 0)
	if err != nil {
		return err
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	minusOne, err := pin(m.ArithmeticIntegerNumeralText("-1"))
	if err != nil {
		return err
	}
	lowerBound, err := pin(m.MkArithmeticGE(expression, minusOne))
	if err != nil {
		return err
	}
	if err := t.assertAxiom(lowerBound); err != nil {
		return err
	}
	zero, err := pin(m.ArithmeticIntegerNumeralText("0"))
	if err != nil {
		return err
	}
	isZero, err := pin(m.MkEq(expression, zero))
	if err != nil {
		return err
	}
	zeroString, err := pin(t.stringLiteral("0"))
	if err != nil {
		return err
	}
	wordIsZero, err := pin(m.MkEq(word, zeroString))
	if err != nil {
		return err
	}
	zeroIff, err := pin(m.MkEq(isZero, wordIsZero))
	if err != nil {
		return err
	}
	if err := t.assertAxiom(zeroIff); err != nil {
		return err
	}
	one, err := pin(m.ArithmeticIntegerNumeralText("1"))
	if err != nil {
		return err
	}
	positive, err := pin(m.MkArithmeticGE(expression, one))
	if err != nil {
		return err
	}
	oneString, err := pin(t.stringLiteral("1"))
	if err != nil {
		return err
	}
	nineString, err := pin(t.stringLiteral("9"))
	if err != nil {
		return err
	}
	firstDigit, err := pin(m.MkRegexRange(oneString, nineString))
	if err != nil {
		return err
	}
	digit, err := pin(m.MkRegexRange(zeroString, nineString))
	if err != nil {
		return err
	}
	digits, err := pin(m.MkRegexStar(digit))
	if err != nil {
		return err
	}
	positiveInteger, err := pin(m.MkRegexConcat(firstDigit, digits))
	if err != nil {
		return err
	}
	membership, err := pin(t.regexMembership(word, positiveInteger))
	if err != nil {
		return err
	}
	negative, err := pin(m.MkNotSimplified(positive))
	if err != nil {
		return err
	}
	axiom, err := pin(m.MkOr(negative, membership))
	if err != nil {
		return err
	}
	return t.assertAxiom(axiom)
}

func (t *SMTStringTheory) instantiateIntToString(node *SMTEnode) (result error) {
	m := t.state.manager
	expression := node.Owner()
	if _, found := t.axiomatized[expression]; found {
		return nil
	}
	t.axiomatized[expression] = struct{}{}
	integer, err := m.GetArg(expression, 0)
	if err != nil {
		return err
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	zero, err := pin(m.ArithmeticIntegerNumeralText("0"))
	if err != nil {
		return err
	}
	nonnegative, err := pin(m.MkArithmeticGE(integer, zero))
	if err != nil {
		return err
	}
	negative, err := pin(m.MkNotSimplified(nonnegative))
	if err != nil {
		return err
	}
	empty, err := pin(t.stringLiteral(""))
	if err != nil {
		return err
	}
	isEmpty, err := pin(m.MkEq(expression, empty))
	if err != nil {
		return err
	}
	axiom, err := pin(m.MkEq(negative, isEmpty))
	if err != nil {
		return err
	}
	return t.assertAxiom(axiom)
}
