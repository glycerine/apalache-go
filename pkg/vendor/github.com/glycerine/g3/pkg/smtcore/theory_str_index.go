package smtcore

// Source: src/smt/theory_str.cpp mk_indexof and instantiate_axiom_Indexof,
// instantiate_axiom_Indexof_extended. Preserve the retained source's tail-relative
// result in the extended case, including its delayed containment heuristic.

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) stringIndexTerm(word, needle ast.ASTHandle) (result ast.ASTHandle, resultErr error) {
	m := t.state.manager
	zero, err := m.ArithmeticIntegerNumeralText("0")
	if err != nil {
		return result, err
	}
	root, err := m.NewRef(zero)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&root)) }()
	result, err = m.MkSequenceIndex(word, needle, zero)
	if err != nil {
		return result, err
	}
	if err := t.retain(result); err != nil {
		return result, err
	}
	if err := t.state.Internalize(result, false); err != nil {
		return result, err
	}
	return result, t.setUpAxioms(result)
}

func (t *SMTStringTheory) instantiateIndex(node *SMTEnode) (result error) {
	m := t.state.manager
	expression := node.Owner()
	if _, found := t.axiomatized[expression]; found {
		return nil
	}
	start, err := m.GetArg(expression, 2)
	if err != nil {
		return err
	}
	text, _, numeral, err := m.ArithmeticNumeralText(start)
	if err != nil {
		return err
	}
	value, parsed := new(big.Rat).SetString(text)
	if !numeral || !parsed || value.Sign() != 0 {
		return t.instantiateIndexExtended(node)
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
	prefix, err := t.freshStringVariable("x1")
	if err != nil {
		return err
	}
	tail, err := t.freshStringVariable("x2")
	if err != nil {
		return err
	}
	index, err := t.freshIntegerVariable("index")
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
	before, err := t.freshStringVariable("x3")
	if err != nil {
		return err
	}
	after, err := t.freshStringVariable("x4")
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
	whole, err = pin(t.concatTerm(before, after))
	if err != nil {
		return err
	}
	if err := items.equate(word, whole); err != nil {
		return err
	}
	length, err := pin(m.MkSequenceLength(before))
	if err != nil {
		return err
	}
	if err := items.equate(length, beforeLength); err != nil {
		return err
	}
	contains, err := pin(t.stringContainsTerm(before, needle))
	if err != nil {
		return err
	}
	negative, err := pin(m.MkNot(contains))
	if err != nil {
		return err
	}
	if err := items.add(negative); err != nil {
		return err
	}
	thenBranch, err := pin(m.MkAndArray(items.values))
	if err != nil {
		return err
	}
	elseBranch, err := pin(m.MkEq(index, minusOne))
	if err != nil {
		return err
	}
	breakdown, err := pin(m.MkIte(condition, thenBranch, elseBranch))
	if err != nil {
		return err
	}
	reduction, err := pin(m.MkEq(expression, index))
	if err != nil {
		return err
	}
	axiom, err := pin(m.MkAnd(breakdown, reduction))
	if err != nil {
		return err
	}
	if err := t.assertAxiom(axiom); err != nil {
		return err
	}
	// Native constructs an unused contains/index equivalence with its assertion
	// commented out. Its live effect is internalizing this contains term.
	return t.state.Internalize(condition, false)
}

func (t *SMTStringTheory) instantiateIndexExtended(node *SMTEnode) (result error) {
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
	start, err := m.GetArg(expression, 2)
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
	imply := func(premise, conclusion ast.ASTHandle) error {
		// Native assert_implication uses the simplifying mk_not(m, ...)
		// helper. Rewriting a literal start can leave a single negated bound;
		// a raw NOT constructor would introduce a double negation here.
		negative, err := pin(m.MkNotSimplified(premise))
		if err != nil {
			return err
		}
		axiom, err := pin(m.MkOr(negative, conclusion))
		if err != nil {
			return err
		}
		return t.assertAxiom(axiom)
	}
	minusOne, err := pin(m.ArithmeticIntegerNumeralText("-1"))
	if err != nil {
		return err
	}
	zero, err := pin(m.ArithmeticIntegerNumeralText("0"))
	if err != nil {
		return err
	}
	negativeStart, err := pin(m.MkArithmeticLE(start, minusOne))
	if err != nil {
		return err
	}
	noMatch, err := pin(m.MkEq(expression, minusOne))
	if err != nil {
		return err
	}
	if err := imply(negativeStart, noMatch); err != nil {
		return fmt.Errorf("extended index negative start: %w", err)
	}
	zeroStart, err := pin(m.MkEq(start, zero))
	if err != nil {
		return err
	}
	simple, err := pin(t.stringIndexTerm(word, needle))
	if err != nil {
		return err
	}
	reduction, err := pin(m.MkEq(expression, simple))
	if err != nil {
		return err
	}
	if err := imply(zeroStart, reduction); err != nil {
		return fmt.Errorf("extended index zero start: %w", err)
	}
	length, err := pin(m.MkSequenceLength(word))
	if err != nil {
		return err
	}
	negativeLength, err := pin(m.MkArithmeticMul(minusOne, length))
	if err != nil {
		return err
	}
	difference, err := pin(m.MkArithmeticAdd(start, negativeLength))
	if err != nil {
		return err
	}
	outside, err := pin(m.MkArithmeticGE(difference, zero))
	if err != nil {
		return err
	}
	if err := imply(outside, noMatch); err != nil {
		return fmt.Errorf("extended index outside start: %w", err)
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	positiveStart, err := pin(m.MkFamilyApp(family, 5, nil, []ast.ASTHandle{start, zero}))
	if err != nil {
		return err
	}
	inside, err := pin(m.MkNot(outside))
	if err != nil {
		return err
	}
	premise, err := pin(m.MkAnd(positiveStart, inside))
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
	root, err := m.NewRef(premise)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&root)) }()
	if err := rw.Rewrite(premise, root); err != nil {
		return err
	}
	premise, err = m.RefGet(root)
	if err != nil {
		return err
	}
	head, err := t.freshStringVariable("hd")
	if err != nil {
		return err
	}
	tail, err := t.freshStringVariable("tl")
	if err != nil {
		return err
	}
	items := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, items.close()) }()
	whole, err := pin(t.concatTerm(head, tail))
	if err != nil {
		return err
	}
	if err := items.equate(word, whole); err != nil {
		return err
	}
	headLength, err := pin(m.MkSequenceLength(head))
	if err != nil {
		return err
	}
	if err := items.equate(headLength, start); err != nil {
		return err
	}
	tailIndex, err := pin(t.stringIndexTerm(tail, needle))
	if err != nil {
		return err
	}
	if err := items.equate(expression, tailIndex); err != nil {
		return err
	}
	conclusion, err := pin(m.MkAndArray(items.values))
	if err != nil {
		return err
	}
	if err := imply(premise, conclusion); err != nil {
		return fmt.Errorf("extended index inside start: %w", err)
	}
	// Unlike mk_contains, the heuristic only internalizes this term; source
	// set_up_axioms is not called here. Its equivalence is search-phase work.
	contains, err := pin(m.MkSequenceContains(word, needle))
	if err != nil {
		return err
	}
	if err := t.state.Internalize(contains, false); err != nil {
		return err
	}
	nonnegative, err := pin(m.MkArithmeticGE(expression, zero))
	if err != nil {
		return err
	}
	heuristic, err := pin(m.MkEq(contains, nonnegative))
	if err != nil {
		return err
	}
	heuristicRoot, err := m.NewRef(heuristic)
	if err != nil {
		return err
	}
	t.delayedAssertions = append(t.delayedAssertions, heuristicRoot)
	return nil
}
