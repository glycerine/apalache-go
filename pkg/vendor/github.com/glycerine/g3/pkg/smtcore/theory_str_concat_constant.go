package smtcore

// Source: src/smt/theory_str.cpp solve_concat_eq_str and
// generate_mutual_exclusion.

import (
	"errors"
	"math/big"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) solveConcatConstant(concat, constant ast.ASTHandle) (result error) {
	m := t.state.manager
	first, second, matched, err := m.SequenceConcatParts(concat)
	if err != nil || !matched {
		return err
	}
	text, literal, err := m.SequenceStringValue(constant)
	if err != nil || !literal {
		return err
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return value, locals.add(value)
	}
	equality, err := pin(m.MkEq(concat, constant))
	if err != nil {
		return err
	}
	guards := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, guards.close()) }()
	if text == "" {
		left, err := pin(m.MkEq(first, constant))
		if err != nil {
			return err
		}
		right, err := pin(m.MkEq(second, constant))
		if err != nil {
			return err
		}
		conclusion, err := pin(m.MkAnd(left, right))
		if err != nil {
			return err
		}
		if err := guards.add(equality); err != nil {
			return err
		}
		return guards.imply(conclusion)
	}
	a, ak, err := t.eqcStringValue(first)
	if err != nil {
		return err
	}
	b, bk, err := t.eqcStringValue(second)
	if err != nil {
		return err
	}
	newConcat := concat
	if a != first || b != second {
		if err := guards.equate(first, a); err != nil {
			return err
		}
		if err := guards.equate(second, b); err != nil {
			return err
		}
		newConcat, err = pin(t.concatTerm(a, b))
		if err != nil {
			return err
		}
		if newConcat != constant {
			conclusion, err := pin(m.MkEq(concat, newConcat))
			if err != nil {
				return err
			}
			if err := guards.imply(conclusion); err != nil {
				return err
			}
		}
	}
	if newConcat == constant {
		return nil
	}
	stillConcat, err := m.IsSequenceConcat(newConcat)
	if err != nil || !stillConcat {
		return err
	}
	if ak && bk {
		// Active mk_concat folds two class constants, so this source case
		// cannot survive the non-concat early return above.
		return nil
	}
	if ak || bk {
		known, variable := a, b
		if bk {
			known, variable = b, a
		}
		piece, _, err := m.SequenceStringValue(known)
		if err != nil {
			return err
		}
		valid := strings.HasPrefix(text, piece)
		if bk {
			valid = strings.HasSuffix(text, piece)
		}
		newEquality, err := pin(m.MkEq(newConcat, constant))
		if err != nil {
			return err
		}
		if !valid {
			negative, err := pin(m.MkNot(newEquality))
			if err != nil {
				return err
			}
			return t.assertAxiom(negative)
		}
		remainder := text[len(piece):]
		if bk {
			remainder = text[:len(text)-len(piece)]
		}
		value, err := pin(t.stringLiteral(remainder))
		if err != nil {
			return err
		}
		conclusion, err := pin(m.MkEq(variable, value))
		if err != nil {
			return err
		}
		premises := smtStringLengthPremises{theory: t}
		defer func() { result = errors.Join(result, premises.close()) }()
		if err := premises.add(newEquality); err != nil {
			return err
		}
		return premises.imply(conclusion)
	}
	// eval_concat cannot succeed when neither operand has a class value.
	al, alk, err := t.lengthValue(a)
	if err != nil {
		return err
	}
	bl, blk, err := t.lengthValue(b)
	if err != nil {
		return err
	}
	if alk || blk {
		return t.solveConcatKnownLength(equality, a, b, text, al, alk, bl, blk)
	}
	return t.solveConcatArrangements(equality, a, b, text)
}

func (t *SMTStringTheory) solveConcatKnownLength(equality, a, b ast.ASTHandle, text string, al *big.Int, alk bool, bl *big.Int, blk bool) (result error) {
	m := t.state.manager
	variable, length := a, al
	if !alk {
		variable, length = b, bl
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return value, locals.add(value)
	}
	term, err := pin(m.MkSequenceLength(variable))
	if err != nil {
		return err
	}
	bound := "0"
	if length.Sign() >= 0 {
		bound = new(big.Int).SetUint64(uint64(len(text))).String()
	}
	numeral, err := pin(m.ArithmeticIntegerNumeralText(bound))
	if err != nil {
		return err
	}
	if length.Sign() < 0 {
		axiom, err := pin(m.MkArithmeticGE(term, numeral))
		if err != nil {
			return err
		}
		return t.assertAxiom(axiom)
	}
	guards := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, guards.close()) }()
	if err := guards.add(equality); err != nil {
		return err
	}
	if length.Cmp(new(big.Int).SetUint64(uint64(len(text)))) > 0 {
		conclusion, err := pin(m.MkArithmeticLE(term, numeral))
		if err != nil {
			return err
		}
		return guards.imply(conclusion)
	}
	split := int(length.Uint64())
	if !alk {
		split = len(text) - split
	}
	for i, expression := range []ast.ASTHandle{a, b} {
		piece := text[:split]
		if i == 1 {
			piece = text[split:]
		}
		concat, err := m.IsSequenceConcat(expression)
		if err != nil {
			return err
		}
		if concat {
			possible, err := t.canConcatEqualString(expression, piece)
			if err != nil {
				return err
			}
			if !possible {
				premise := smtStringLengthPremises{theory: t}
				defer func() { result = errors.Join(result, premise.close()) }()
				if err := premise.length(variable, length); err != nil {
					return err
				}
				negative, err := pin(m.MkNot(premise.values[0]))
				if err != nil {
					return err
				}
				return guards.imply(negative)
			}
		}
	}
	if err := guards.length(variable, length); err != nil {
		return err
	}
	conclusions := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, conclusions.close()) }()
	for i, expression := range []ast.ASTHandle{a, b} {
		piece := text[:split]
		if i == 1 {
			piece = text[split:]
		}
		value, err := pin(t.stringLiteral(piece))
		if err != nil {
			return err
		}
		eq, err := pin(m.MkEq(expression, value))
		if err != nil {
			return err
		}
		if err := conclusions.add(eq); err != nil {
			return err
		}
	}
	for i, expression := range []ast.ASTHandle{a, b} {
		piece, known := text[:split], alk
		if i == 1 {
			piece, known = text[split:], blk
		}
		if !known {
			if err := conclusions.length(expression, new(big.Int).SetUint64(uint64(len(piece)))); err != nil {
				return err
			}
		}
	}
	conclusion, err := pin(m.MkAndArray(conclusions.values))
	if err != nil {
		return err
	}
	return guards.imply(conclusion)
}

func (t *SMTStringTheory) solveConcatArrangements(equality, a, b ast.ASTHandle, text string) (result error) {
	m := t.state.manager
	key, reverse := [2]ast.ASTHandle{a, b}, [2]ast.ASTHandle{b, a}
	if _, first := t.concatBreakFlags[key]; !first {
		if _, second := t.concatBreakFlags[reverse]; !second {
			flag, err := t.freshIntegerVariable("$$_xor")
			if err != nil {
				return err
			}
			t.concatBreakFlags[key] = flag
		}
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return value, locals.add(value)
	}
	var arrangements []ast.ASTHandle
	for split := 0; split <= len(text); split++ {
		possible := true
		for i, expression := range []ast.ASTHandle{a, b} {
			piece := text[:split]
			if i == 1 {
				piece = text[split:]
			}
			concat, err := m.IsSequenceConcat(expression)
			if err != nil {
				return err
			}
			if concat {
				valid, err := t.canConcatEqualString(expression, piece)
				if err != nil {
					return err
				}
				if !valid {
					possible = false
					break
				}
			}
		}
		if !possible {
			continue
		}
		prefix, err := pin(t.stringLiteral(text[:split]))
		if err != nil {
			return err
		}
		left, err := pin(m.MkEq(a, prefix))
		if err != nil {
			return err
		}
		suffix, err := pin(t.stringLiteral(text[split:]))
		if err != nil {
			return err
		}
		right, err := pin(m.MkEq(b, suffix))
		if err != nil {
			return err
		}
		arrangement, err := pin(m.MkAnd(left, right))
		if err != nil {
			return err
		}
		arrangements = append(arrangements, arrangement)
	}
	if len(arrangements) == 0 {
		negative, err := pin(m.MkNot(equality))
		if err != nil {
			return err
		}
		return t.assertAxiom(negative)
	}
	disjunction, err := pin(m.MkOrArray(arrangements))
	if err != nil {
		return err
	}
	strong := true
	if t.parameterOwner != nil {
		parameters, err := t.stringParameterState()
		if err != nil {
			return err
		}
		strong = parameters.StrongArrangements
	}
	if strong {
		axiom, err := pin(m.MkEq(equality, disjunction))
		if err != nil {
			return err
		}
		if err := t.assertAxiom(axiom); err != nil {
			return err
		}
	} else {
		guard := smtStringLengthPremises{theory: t}
		defer func() { result = errors.Join(result, guard.close()) }()
		if err := guard.add(equality); err != nil {
			return err
		}
		if err := guard.imply(disjunction); err != nil {
			return err
		}
	}
	literals := make([]SMTLiteral, len(arrangements))
	for i, expression := range arrangements {
		literal, err := t.state.Literal(expression)
		if err != nil {
			return err
		}
		literals[i] = literal
	}
	return t.state.MakeTheoryCaseSplit(literals)
}
