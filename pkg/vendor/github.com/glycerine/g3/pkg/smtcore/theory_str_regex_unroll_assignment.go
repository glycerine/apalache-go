package smtcore

// Source: src/smt/theory_str.cpp gen_assign_unroll_reg.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) assignStringRegexUnrolls(unrolls []ast.ASTHandle) error {
	ordered := append([]ast.ASTHandle(nil), unrolls...)
	stringSortHandles(ordered)
	var previous ast.ASTHandle
	for i, term := range ordered {
		if i > 0 && term == previous {
			continue
		}
		previous = term
		if err := t.assignStringRegexUnroll(term); err != nil {
			return err
		}
	}
	return nil
}

func (t *SMTStringTheory) assignStringRegexUnroll(unroll ast.ASTHandle) (result error) {
	m := t.state.manager
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
	low, known, err := t.stringArithmeticBound(count, true)
	if err != nil {
		return err
	}
	// The unused high endpoint still has an actual native reader call.
	if _, _, err = t.stringArithmeticBound(count, false); err != nil {
		return err
	}
	locals := smtStringLengthPremises{theory: t}
	items := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, items.close(), locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	lengthOf := func(term ast.ASTHandle) (ast.ASTHandle, error) { return pin(m.MkSequenceLength(term)) }
	addEq := func(left, right ast.ASTHandle) error {
		equal, err := t.state.mkEqualityAtom(left, right)
		if err != nil {
			return err
		}
		return items.add(equal)
	}
	var formula ast.ASTHandle
	if known && low.Sign() < 0 {
		zero, err := pin(m.ArithmeticIntegerNumeralText("0"))
		if err != nil {
			return err
		}
		formula, err = pin(m.MkArithmeticGE(count, zero))
		if err != nil {
			return err
		}
	} else if cached, ok := t.testers.unrollDefinitions[unroll]; ok {
		formula = cached
	} else {
		first, err := t.freshRegexVariable()
		if err != nil {
			return err
		}
		second, err := t.freshRegexVariable()
		if err != nil {
			return err
		}
		concat, err := pin(t.concatTerm(first, second))
		if err != nil {
			return err
		}
		nextCount, err := t.freshUnrollBound()
		if err != nil {
			return err
		}
		nextUnroll, err := pin(t.makeStringUnroll(regex, nextCount))
		if err != nil {
			return err
		}
		if err = addEq(unroll, concat); err != nil {
			return err
		}
		length, err := lengthOf(unroll)
		if err != nil {
			return err
		}
		firstLength, err := lengthOf(first)
		if err != nil {
			return err
		}
		secondLength, err := lengthOf(second)
		if err != nil {
			return err
		}
		sum, err := pin(m.MkArithmeticAdd(firstLength, secondLength))
		if err != nil {
			return err
		}
		if err = addEq(length, sum); err != nil {
			return err
		}
		for _, part := range []ast.ASTHandle{firstLength, secondLength} {
			minus, err := pin(m.ArithmeticIntegerNumeralText("-1"))
			if err != nil {
				return err
			}
			negative, err := pin(m.MkArithmeticMul(minus, part))
			if err != nil {
				return err
			}
			difference, err := pin(m.MkArithmeticAdd(length, negative))
			if err != nil {
				return err
			}
			zero, err := pin(m.ArithmeticIntegerNumeralText("0"))
			if err != nil {
				return err
			}
			bound, err := pin(m.MkArithmeticGE(difference, zero))
			if err != nil {
				return err
			}
			if err = items.add(bound); err != nil {
				return err
			}
		}
		if err = t.reduceStringVirtualRegexIn(first, regex, &items); err != nil {
			return err
		}
		one, err := pin(m.ArithmeticIntegerNumeralText("1"))
		if err != nil {
			return err
		}
		successor, err := pin(m.MkArithmeticAdd(nextCount, one))
		if err != nil {
			return err
		}
		if err = addEq(count, successor); err != nil {
			return err
		}
		if err = addEq(second, nextUnroll); err != nil {
			return err
		}
		nextLength, err := lengthOf(nextUnroll)
		if err != nil {
			return err
		}
		if err = addEq(secondLength, nextLength); err != nil {
			return err
		}
		body, err := pin(m.MkAndArray(items.values))
		if err != nil {
			return err
		}
		positive, err := pin(m.MkArithmeticGE(count, one))
		if err != nil {
			return err
		}
		positiveCase, err := pin(t.state.mkEqualityAtom(positive, body))
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
		empty, err := pin(t.stringLiteral(""))
		if err != nil {
			return err
		}
		emptyCase, err := pin(t.state.mkEqualityAtom(unroll, empty))
		if err != nil {
			return err
		}
		lengthZero, err := pin(t.state.mkEqualityAtom(length, zero))
		if err != nil {
			return err
		}
		zeroBody, err := pin(m.MkAnd(emptyCase, lengthZero))
		if err != nil {
			return err
		}
		zeroEquivalent, err := pin(t.state.mkEqualityAtom(zeroCase, zeroBody))
		if err != nil {
			return err
		}
		formula, err = pin(m.MkAnd(zeroEquivalent, positiveCase))
		if err != nil {
			return err
		}
		t.testers.unrollDefinitions[unroll] = formula
	}
	if err = t.retain(formula); err != nil {
		return err
	}
	return t.assertAxiom(formula)
}
