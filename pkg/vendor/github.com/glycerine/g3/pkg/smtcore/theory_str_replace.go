package smtcore

// Source: src/smt/theory_str.cpp mk_contains and instantiate_axiom_Replace.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) stringContainsTerm(haystack, needle ast.ASTHandle) (ast.ASTHandle, error) {
	expression, err := t.state.manager.MkSequenceContains(haystack, needle)
	if err != nil {
		return expression, err
	}
	if err := t.retain(expression); err != nil {
		return expression, err
	}
	if err := t.state.Internalize(expression, false); err != nil {
		return expression, err
	}
	return expression, t.setUpAxioms(expression)
}

func (t *SMTStringTheory) instantiateReplace(node *SMTEnode) (result error) {
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
	needle, err := m.GetArg(expression, 1)
	if err != nil {
		return err
	}
	replacement, err := m.GetArg(expression, 2)
	if err != nil {
		return err
	}
	prefix, err := t.freshStringVariable("x1")
	if err != nil {
		return err
	}
	tail, err := t.freshStringVariable("x2")
	if err != nil {
		return err
	}
	index, err := t.freshIntegerVariable("i1")
	if err != nil {
		return err
	}
	value, err := t.freshStringVariable("result")
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
	condition, err := pin(t.stringContainsTerm(word, needle))
	if err != nil {
		return err
	}
	items := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, items.close()) }()
	rest, err := pin(t.concatTerm(needle, tail))
	if err != nil {
		return err
	}
	whole, err := pin(t.concatTerm(prefix, rest))
	if err != nil {
		return err
	}
	if err := items.equate(word, whole); err != nil {
		return err
	}
	prefixLength, err := pin(m.MkSequenceLength(prefix))
	if err != nil {
		return err
	}
	if err := items.equate(index, prefixLength); err != nil {
		return err
	}
	beforeMatch, err := t.freshStringVariable("x3")
	if err != nil {
		return err
	}
	fromMatch, err := t.freshStringVariable("x4")
	if err != nil {
		return err
	}
	needleLength, err := pin(m.MkSequenceLength(needle))
	if err != nil {
		return err
	}
	minusOne, err := pin(m.ArithmeticIntegerNumeralText("-1"))
	if err != nil {
		return err
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	beforeLength, err := pin(m.MkFamilyApp(family, 6, nil, []ast.ASTHandle{index, needleLength, minusOne}))
	if err != nil {
		return err
	}
	whole, err = pin(t.concatTerm(beforeMatch, fromMatch))
	if err != nil {
		return err
	}
	if err := items.equate(word, whole); err != nil {
		return err
	}
	length, err := pin(m.MkSequenceLength(beforeMatch))
	if err != nil {
		return err
	}
	if err := items.equate(length, beforeLength); err != nil {
		return err
	}
	earlierMatch, err := pin(t.stringContainsTerm(beforeMatch, needle))
	if err != nil {
		return err
	}
	noEarlierMatch, err := pin(m.MkNot(earlierMatch))
	if err != nil {
		return err
	}
	if err := items.add(noEarlierMatch); err != nil {
		return err
	}
	rest, err = pin(t.concatTerm(replacement, tail))
	if err != nil {
		return err
	}
	newWord, err := pin(t.concatTerm(prefix, rest))
	if err != nil {
		return err
	}
	if err := items.equate(value, newWord); err != nil {
		return err
	}
	thenBranch, err := pin(m.MkAndArray(items.values))
	if err != nil {
		return err
	}
	elseBranch, err := pin(m.MkEq(value, word))
	if err != nil {
		return err
	}
	breakdown, err := pin(m.MkIte(condition, thenBranch, elseBranch))
	if err != nil {
		return err
	}
	if t.state.theoryRewriter == nil {
		return ast.ErrASTHandle
	}
	rw, err := t.state.theoryRewriter.FreshDefault()
	if rw != nil {
		defer func() { result = errors.Join(result, rw.Close()) }()
	}
	if err != nil {
		return err
	}
	if err := t.rewriteStringAxiom(breakdown, rw); err != nil {
		return err
	}
	reduction, err := pin(m.MkEq(expression, value))
	if err != nil {
		return err
	}
	return t.rewriteStringAxiom(reduction, rw)
}
