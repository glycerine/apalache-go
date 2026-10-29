package smtcore

// Source: src/smt/theory_str.cpp get_eqc_allUnroll,
// process_unroll_eq_const_str and unroll_str2reg_constStr.

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) stringClassUnrolls(expression ast.ASTHandle) (constant ast.ASTHandle, unrolls []ast.ASTHandle, result error) {
	m := t.state.manager
	seen := make(map[ast.ASTHandle]struct{})
	for _, member := range t.stringClass(expression) {
		if _, literal, err := m.SequenceStringValue(member); err != nil {
			return constant, unrolls, err
		} else if literal {
			// Native collection returns the last constant in cycle order.
			constant = member
			continue
		}
		op, own, err := t.operation(member)
		if err != nil {
			return constant, unrolls, err
		}
		if own && op == 43 {
			if _, found := seen[member]; !found {
				seen[member] = struct{}{}
				unrolls = append(unrolls, member)
			}
		}
	}
	stringSortHandles(unrolls)
	return constant, unrolls, nil
}

func (t *SMTStringTheory) processEqualityUnrollConstants(left, right ast.ASTHandle) error {
	lc, lu, err := t.stringClassUnrolls(left)
	if err != nil {
		return err
	}
	rc, ru, err := t.stringClassUnrolls(right)
	if err != nil {
		return err
	}
	if !rc.IsNull() {
		for _, unroll := range lu {
			if err := t.processUnrollConstant(unroll, rc); err != nil {
				return err
			}
		}
	}
	if !lc.IsNull() {
		for _, unroll := range ru {
			if err := t.processUnrollConstant(unroll, lc); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *SMTStringTheory) processUnrollConstant(unroll, constant ast.ASTHandle) (result error) {
	m := t.state.manager
	op, own, err := t.operation(unroll)
	if err != nil || !own || op != 43 {
		return err
	}
	text, literal, err := m.SequenceStringValue(constant)
	if err != nil || !literal || text == "" {
		return err
	}
	regex, err := m.GetArg(unroll, 0)
	if err != nil {
		return err
	}
	op, own, err = t.operation(regex)
	if err != nil || !own || op != 11 {
		return err
	}
	pattern, err := m.GetArg(regex, 0)
	if err != nil {
		return err
	}
	patternText, patternLiteral, err := m.SequenceStringValue(pattern)
	if err != nil {
		return err
	}
	// Source asserts a nonempty literal here and divides by its byte count.
	// Do not replace the quotient with a divisibility or matching check.
	patternBytes := uint32(len(patternText))
	if !patternLiteral || patternBytes == 0 {
		return ast.ErrASTDomain
	}
	count, err := m.GetArg(unroll, 1)
	if err != nil {
		return err
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(expression ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return expression, locals.add(expression)
	}
	guard, err := pin(m.MkEq(unroll, constant))
	if err != nil {
		return err
	}
	quotient, err := pin(m.ArithmeticIntegerNumeralText(strconv.FormatUint(uint64(uint32(len(text))/patternBytes), 10)))
	if err != nil {
		return err
	}
	countEquality, err := pin(m.MkEq(count, quotient))
	if err != nil {
		return err
	}
	length, err := pin(m.MkSequenceLength(unroll))
	if err != nil {
		return err
	}
	byteCount, err := pin(m.ArithmeticIntegerNumeralText(strconv.FormatUint(uint64(uint32(len(text))), 10)))
	if err != nil {
		return err
	}
	lengthEquality, err := pin(m.MkEq(length, byteCount))
	if err != nil {
		return err
	}
	conclusion, err := pin(m.MkAnd(countEquality, lengthEquality))
	if err != nil {
		return err
	}
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	if err := premises.add(guard); err != nil {
		return err
	}
	return premises.imply(conclusion)
}
