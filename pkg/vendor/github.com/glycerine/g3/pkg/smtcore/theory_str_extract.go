package smtcore

// Source: src/smt/theory_str.cpp instantiate_axiom_CharAt and
// instantiate_axiom_Substr. These use context/default rewriters respectively.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func (t *SMTStringTheory) rewriteStringAxiom(expression ast.ASTHandle, rw *rewriter.TheoryRewriter) (result error) {
	if rw == nil {
		return ast.ErrASTHandle // A native SMT context always owns a rewriter.
	}
	m := t.state.manager
	root, err := m.NewRef(expression)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&root)) }()
	if err := rw.Rewrite(expression, root); err != nil {
		return err
	}
	value, err := m.RefGet(root)
	if err != nil {
		return err
	}
	return t.assertAxiom(value)
}

func (t *SMTStringTheory) instantiateCharAt(node *SMTEnode) (result error) {
	m := t.state.manager
	expression := node.Owner()
	if _, found := t.axiomatized[expression]; found {
		return nil
	}
	t.axiomatized[expression] = struct{}{}
	word, index, matched, err := m.SequenceBinaryOperationArguments(expression, 8)
	if err != nil || !matched {
		if err == nil {
			err = ast.ErrASTDomain
		}
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
	variables := [3]ast.ASTHandle{}
	for i, name := range []string{"ts0", "ts1", "ts2"} {
		variables[i], err = t.freshStringVariable(name)
		if err != nil {
			return err
		}
	}
	zero, err := pin(m.ArithmeticIntegerNumeralText("0"))
	if err != nil {
		return err
	}
	nonnegative, err := pin(m.MkArithmeticGE(index, zero))
	if err != nil {
		return err
	}
	length, err := pin(m.MkSequenceLength(word))
	if err != nil {
		return err
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	belowLength, err := pin(m.MkFamilyApp(family, 4, nil, []ast.ASTHandle{index, length}))
	if err != nil {
		return err
	}
	condition, err := pin(m.MkAnd(nonnegative, belowLength))
	if err != nil {
		return err
	}
	tail, err := pin(t.concatTerm(variables[1], variables[2]))
	if err != nil {
		return err
	}
	whole, err := pin(t.concatTerm(variables[0], tail))
	if err != nil {
		return err
	}
	items := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, items.close()) }()
	if err := items.equate(word, whole); err != nil {
		return err
	}
	prefixLength, err := pin(m.MkSequenceLength(variables[0]))
	if err != nil {
		return err
	}
	if err := items.equate(index, prefixLength); err != nil {
		return err
	}
	if err := items.length(variables[1], big.NewInt(1)); err != nil {
		return err
	}
	thenBranch, err := pin(m.MkAndArray(items.values))
	if err != nil {
		return err
	}
	empty, err := pin(t.stringLiteral(""))
	if err != nil {
		return err
	}
	elseBranch, err := pin(m.MkEq(variables[1], empty))
	if err != nil {
		return err
	}
	choice, err := pin(m.MkIte(condition, thenBranch, elseBranch))
	if err != nil {
		return err
	}
	reduction, err := pin(m.MkEq(expression, variables[1]))
	if err != nil {
		return err
	}
	final, err := pin(m.MkAnd(choice, reduction))
	if err != nil {
		return err
	}
	return t.rewriteStringAxiom(final, t.state.theoryRewriter)
}

func (t *SMTStringTheory) instantiateSubstring(node *SMTEnode) (result error) {
	m := t.state.manager
	expression := node.Owner()
	if _, found := t.axiomatized[expression]; found {
		return nil
	}
	t.axiomatized[expression] = struct{}{}
	op, own, err := t.operation(expression)
	if err != nil || !own || op != 6 {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return err
	}
	word, err := m.GetArg(expression, 0)
	if err != nil {
		return err
	}
	position, err := m.GetArg(expression, 1)
	if err != nil {
		return err
	}
	count, err := m.GetArg(expression, 2)
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
	minusOne, err := pin(m.ArithmeticIntegerNumeralText("-1"))
	if err != nil {
		return err
	}
	length, err := pin(m.MkSequenceLength(word))
	if err != nil {
		return err
	}
	negativeLength, err := pin(m.MkArithmeticMul(minusOne, length))
	if err != nil {
		return err
	}
	difference, err := pin(m.MkArithmeticAdd(position, negativeLength))
	if err != nil {
		return err
	}
	outside, err := pin(m.MkArithmeticGE(difference, zero))
	if err != nil {
		return err
	}
	inside, err := pin(m.MkNot(outside))
	if err != nil {
		return err
	}
	positionValid, err := pin(m.MkArithmeticGE(position, zero))
	if err != nil {
		return err
	}
	countValid, err := pin(m.MkArithmeticGE(count, zero))
	if err != nil {
		return err
	}
	valid, err := pin(m.MkAndArray([]ast.ASTHandle{positionValid, inside, countValid}))
	if err != nil {
		return err
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	endDifference, err := pin(m.MkFamilyApp(family, 6, nil, []ast.ASTHandle{position, count, negativeLength}))
	if err != nil {
		return err
	}
	endOutside, err := pin(m.MkArithmeticGE(endDifference, zero))
	if err != nil {
		return err
	}
	invalid, err := pin(m.MkNot(valid))
	if err != nil {
		return err
	}
	empty, err := pin(t.stringLiteral(""))
	if err != nil {
		return err
	}
	emptyResult, err := pin(m.MkEq(expression, empty))
	if err != nil {
		return err
	}
	case1, err := pin(m.MkImplies(invalid, emptyResult))
	if err != nil {
		return err
	}
	variables := [5]ast.ASTHandle{}
	for i, name := range []string{"t0", "t1"} {
		variables[i], err = t.freshStringVariable(name)
		if err != nil {
			return err
		}
	}
	wide := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, wide.close()) }()
	whole, err := pin(t.concatTerm(variables[0], variables[1]))
	if err != nil {
		return err
	}
	if err := wide.equate(word, whole); err != nil {
		return err
	}
	prefixLength, err := pin(m.MkSequenceLength(variables[0]))
	if err != nil {
		return err
	}
	if err := wide.equate(prefixLength, position); err != nil {
		return err
	}
	if err := wide.equate(expression, variables[1]); err != nil {
		return err
	}
	wideConclusion, err := pin(m.MkAndArray(wide.values))
	if err != nil {
		return err
	}
	wideGuard, err := pin(m.MkAnd(valid, endOutside))
	if err != nil {
		return err
	}
	case2, err := pin(m.MkImplies(wideGuard, wideConclusion))
	if err != nil {
		return err
	}
	for i, name := range []string{"t2", "t3", "t4"} {
		variables[i+2], err = t.freshStringVariable(name)
		if err != nil {
			return err
		}
	}
	narrow := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, narrow.close()) }()
	tail, err := pin(t.concatTerm(variables[3], variables[4]))
	if err != nil {
		return err
	}
	whole, err = pin(t.concatTerm(variables[2], tail))
	if err != nil {
		return err
	}
	if err := narrow.equate(word, whole); err != nil {
		return err
	}
	prefixLength, err = pin(m.MkSequenceLength(variables[2]))
	if err != nil {
		return err
	}
	if err := narrow.equate(prefixLength, position); err != nil {
		return err
	}
	resultLength, err := pin(m.MkSequenceLength(variables[3]))
	if err != nil {
		return err
	}
	if err := narrow.equate(resultLength, count); err != nil {
		return err
	}
	if err := narrow.equate(expression, variables[3]); err != nil {
		return err
	}
	narrowConclusion, err := pin(m.MkAndArray(narrow.values))
	if err != nil {
		return err
	}
	endInside, err := pin(m.MkNot(endOutside))
	if err != nil {
		return err
	}
	narrowGuard, err := pin(m.MkAnd(valid, endInside))
	if err != nil {
		return err
	}
	case3, err := pin(m.MkImplies(narrowGuard, narrowConclusion))
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
	for _, axiom := range []ast.ASTHandle{case1, case2, case3} {
		if err := t.rewriteStringAxiom(axiom, rw); err != nil {
			return err
		}
	}
	return nil
}
