package smtcore

// Source: src/smt/theory_str.cpp finalcheck_str2int/finalcheck_int2str.
// These are the retained conversion-agreement stage; the enclosing final-check
// pipeline must establish its source prerequisites before dispatching them.

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func stringDecimalValue(text string) (*big.Int, bool) {
	value := new(big.Int)
	if len(text) == 0 {
		return value, false
	}
	ten := big.NewInt(10)
	for i := 0; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			return value, false
		}
		value.Mul(value, ten)
		value.Add(value, new(big.Int).SetUint64(uint64(text[i]-'0')))
	}
	return value, true
}

func (t *SMTStringTheory) assertConversionAgreement(axiom ast.ASTHandle) (bool, error) {
	if _, found := t.conversionAxioms[axiom]; found {
		return false, nil
	}
	// Native insert_obj_trail removes only the set membership at scope pop;
	// assert_axiom's lifetime trail continues to own the expression.
	t.conversionAxioms[axiom] = t.level
	return true, t.assertAxiom(axiom)
}

func (t *SMTStringTheory) finalCheckStringToInt(expression ast.ASTHandle) (added bool, result error) {
	m := t.state.manager
	word, err := m.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	publish := func(premise, conclusion ast.ASTHandle) error {
		negative, err := pin(m.MkNotSimplified(premise))
		if err != nil {
			return err
		}
		axiom, err := pin(m.MkOr(negative, conclusion))
		if err != nil {
			return err
		}
		changed, err := t.assertConversionAgreement(axiom)
		added = added || changed
		return err
	}
	integer, known, err := t.arithmeticValue(expression)
	if err != nil {
		return added, err
	}
	if known {
		if integer.Cmp(big.NewInt(-1)) != 0 {
			value, err := pin(m.ArithmeticIntegerNumeralText(integer.String()))
			if err != nil {
				return added, err
			}
			premise, err := pin(m.MkEq(expression, value))
			if err != nil {
				return added, err
			}
			text, err := pin(t.stringLiteral(integer.String()))
			if err != nil {
				return added, err
			}
			conclusion, err := pin(m.MkEq(word, text))
			if err != nil {
				return added, err
			}
			if err := publish(premise, conclusion); err != nil {
				return added, err
			}
		}
	} else {
		zero, err := pin(m.ArithmeticIntegerNumeralText("0"))
		if err != nil {
			return added, err
		}
		isZero, err := pin(m.MkEq(expression, zero))
		if err != nil {
			return added, err
		}
		if _, err := t.ensureEnode(isZero); err != nil {
			return added, err
		}
		// Source mk_literal creates the zero equality without asserting it,
		// then requests continuation even if that equality already existed.
		if _, err := t.state.Literal(isZero); err != nil {
			return added, err
		}
		added = true
	}
	constant, known, err := t.eqcStringValue(word)
	if err != nil || !known {
		return added, err
	}
	text, _, err := m.SequenceStringValue(constant)
	if err != nil {
		return added, err
	}
	converted, valid := stringDecimalValue(text)
	if !valid {
		converted.SetInt64(-1)
	}
	premise, err := pin(m.MkEq(word, constant))
	if err != nil {
		return added, err
	}
	value, err := pin(m.ArithmeticIntegerNumeralText(converted.String()))
	if err != nil {
		return added, err
	}
	conclusion, err := pin(m.MkEq(expression, value))
	if err != nil {
		return added, err
	}
	err = publish(premise, conclusion)
	return added, err
}

func (t *SMTStringTheory) finalCheckIntToString(expression ast.ASTHandle) (added bool, result error) {
	m := t.state.manager
	integer, err := m.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	constant, known, err := t.eqcStringValue(expression)
	if err != nil {
		return false, err
	}
	if !known {
		// Source NOT_IMPLEMENTED_YET: the enclosing stage expects an assigned
		// String class and must not accept an unassigned conversion as done.
		return false, fmt.Errorf("string conversion agreement requires a concrete int-to-string class value")
	}
	text, _, err := m.SequenceStringValue(constant)
	if err != nil || len(text) == 0 {
		return false, err // Empty result is covered by the initial axiom.
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	premise, err := pin(m.MkEq(expression, constant))
	if err != nil {
		return false, err
	}
	converted, valid := stringDecimalValue(text)
	if !valid {
		// Native deliberately does not memoize this conflict clause.
		axiom, err := pin(m.MkNot(premise))
		if err != nil {
			return false, err
		}
		return true, t.assertAxiom(axiom)
	}
	value, err := pin(m.ArithmeticIntegerNumeralText(converted.String()))
	if err != nil {
		return false, err
	}
	conclusion, err := pin(m.MkEq(integer, value))
	if err != nil {
		return false, err
	}
	negative, err := pin(m.MkNotSimplified(premise))
	if err != nil {
		return false, err
	}
	axiom, err := pin(m.MkOr(negative, conclusion))
	if err != nil {
		return false, err
	}
	return t.assertConversionAgreement(axiom)
}
