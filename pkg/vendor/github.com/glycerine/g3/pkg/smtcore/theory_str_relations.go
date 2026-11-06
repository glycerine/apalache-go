package smtcore

// Source: src/smt/theory_str.cpp instantiate_axiom_prefixof,
// instantiate_axiom_suffixof and instantiate_axiom_Contains.

import (
	"errors"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) instantiateBoundaryRelation(node *SMTEnode, suffix bool) (result error) {
	m := t.state.manager
	expression := node.Owner()
	if _, found := t.axiomatized[expression]; found {
		return nil
	}
	t.axiomatized[expression] = struct{}{}
	var refs []ast.ASTRef
	pin := func(expression ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := m.NewRef(expression)
		if err == nil {
			refs = append(refs, ref)
		}
		return expression, err
	}
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			result = errors.Join(result, m.DelRef(&refs[i]))
		}
	}()
	first, err := t.freshStringVariable("ts0")
	if err != nil {
		return err
	}
	second, err := t.freshStringVariable("ts1")
	if err != nil {
		return err
	}
	boundary, err := m.GetArg(expression, 0)
	if err != nil {
		return err
	}
	container, err := m.GetArg(expression, 1)
	if err != nil {
		return err
	}
	word, err := t.concatTerm(first, second)
	if err != nil {
		return err
	}
	wordEquality, err := pin(m.MkEq(container, word))
	if err != nil {
		return err
	}
	segment := first
	if suffix {
		segment = second
	}
	segmentLength, err := pin(m.MkSequenceLength(segment))
	if err != nil {
		return err
	}
	boundaryLength, err := pin(m.MkSequenceLength(boundary))
	if err != nil {
		return err
	}
	lengthEquality, err := pin(m.MkEq(segmentLength, boundaryLength))
	if err != nil {
		return err
	}
	segmentEquality, err := pin(m.MkEq(segment, boundary))
	if err != nil {
		return err
	}
	negative, err := pin(m.MkNot(expression))
	if err != nil {
		return err
	}
	choice, err := pin(m.MkIte(segmentEquality, expression, negative))
	if err != nil {
		return err
	}
	conjunction, err := pin(m.MkAndArray([]ast.ASTHandle{wordEquality, lengthEquality, choice}))
	if err != nil {
		return err
	}
	containerLength, err := pin(m.MkSequenceLength(container))
	if err != nil {
		return err
	}
	minusOne, err := pin(m.ArithmeticIntegerNumeralText("-1"))
	if err != nil {
		return err
	}
	negatedLength, err := pin(m.MkArithmeticMul(minusOne, boundaryLength))
	if err != nil {
		return err
	}
	difference, err := pin(m.MkArithmeticAdd(containerLength, negatedLength))
	if err != nil {
		return err
	}
	zero, err := pin(m.ArithmeticIntegerNumeralText("0"))
	if err != nil {
		return err
	}
	condition, err := pin(m.MkArithmeticGE(difference, zero))
	if err != nil {
		return err
	}
	axiom, err := pin(m.MkIte(condition, conjunction, negative))
	if err != nil {
		return err
	}
	return t.assertAxiom(axiom)
}

func (t *SMTStringTheory) instantiateContains(node *SMTEnode) error {
	m := t.state.manager
	expression := node.Owner()
	if _, found := t.axiomatized[expression]; found {
		return nil
	}
	t.axiomatized[expression] = struct{}{}
	haystack, needle, matched, err := m.SequenceBinaryOperationArguments(expression, 5)
	if err != nil || !matched {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return err
	}
	haystackText, haystackConstant, err := m.SequenceStringValue(haystack)
	if err != nil {
		return err
	}
	needleText, needleConstant, err := m.SequenceStringValue(needle)
	if err != nil {
		return err
	}
	if haystackConstant && needleConstant {
		if strings.Contains(haystackText, needleText) {
			return t.assertAxiom(expression)
		}
		negative, err := m.MkNot(expression)
		if err != nil {
			return err
		}
		return t.assertAxiom(negative)
	}
	ref, err := m.NewRef(expression)
	if err != nil {
		return err
	}
	t.containsRefs = append(t.containsRefs, ref)
	t.contains = append(t.contains, expression)
	key := [2]ast.ASTHandle{haystack, needle}
	t.containsByPair[key] = expression
	for _, argument := range []ast.ASTHandle{haystack, needle} {
		if t.containsIndex[argument] == nil {
			t.containsIndex[argument] = make(map[[2]ast.ASTHandle]struct{})
		}
		t.containsIndex[argument][key] = struct{}{}
	}
	first, err := t.freshStringVariable("ts0")
	if err != nil {
		return err
	}
	second, err := t.freshStringVariable("ts1")
	if err != nil {
		return err
	}
	tail, err := t.concatTerm(needle, second)
	if err != nil {
		return err
	}
	word, err := t.concatTerm(first, tail)
	if err != nil {
		return err
	}
	equality, err := m.MkEq(haystack, word)
	if err != nil {
		return err
	}
	equivalence, err := m.MkEq(expression, equality)
	if err != nil {
		return err
	}
	return t.assertAxiom(equivalence)
}
