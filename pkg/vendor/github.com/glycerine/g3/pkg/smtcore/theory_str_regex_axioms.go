package smtcore

// Source: src/smt/theory_str.cpp mk_regex_rep_var, mk_int_var,
// mk_unroll_bound_var, mk_unroll and instantiate_axiom_RegexIn.

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) freshRegexVariable() (ast.ASTHandle, error) {
	variable, err := t.freshStringVariable("regex")
	if err == nil {
		delete(t.internalVariables, variable)
		t.regexVariables[variable] = struct{}{}
	}
	return variable, err
}

func (t *SMTStringTheory) freshUnrollBound() (ast.ASTHandle, error) {
	return t.freshIntegerVariable("unroll")
}

func (t *SMTStringTheory) freshIntegerVariable(prefix string) (ast.ASTHandle, error) {
	m := t.state.manager
	zero, err := m.ArithmeticIntegerNumeralText("0")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := m.GetSort(zero)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	name := prefix + "!tmp" + strconv.FormatUint(uint64(t.freshID), 10)
	t.freshID++
	symbol, err := m.Symbols.Intern(name)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	variable, err := m.MkSequenceSkolem(symbol, nil, sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err := t.state.Internalize(variable, false); err != nil {
		return ast.ASTHandle{}, err
	}
	if err := t.state.markRelevant(variable); err != nil {
		return ast.ASTHandle{}, err
	}
	return variable, t.retain(variable)
}

func (t *SMTStringTheory) instantiateLegacyRegexAxiom(expression, word, regex ast.ASTHandle) (result error) {
	m := t.state.manager
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
	op, _, err := t.operation(regex)
	if err != nil {
		return err
	}
	var axiom ast.ASTHandle
	switch op {
	case 11:
		literal, err := m.GetArg(regex, 0)
		if err != nil {
			return err
		}
		equality, err := pin(m.MkEq(word, literal))
		if err != nil {
			return err
		}
		axiom, err = pin(m.MkIff(expression, equality))
		if err != nil {
			return err
		}
	case 17, 18:
		first, err := t.freshRegexVariable()
		if err != nil {
			return err
		}
		second, err := t.freshRegexVariable()
		if err != nil {
			return err
		}
		var equality ast.ASTHandle
		if op == 17 {
			concat, err := t.concatTerm(first, second)
			if err != nil {
				return err
			}
			equality, err = pin(m.MkEq(word, concat))
			if err != nil {
				return err
			}
		} else {
			left, err := pin(m.MkEq(word, first))
			if err != nil {
				return err
			}
			right, err := pin(m.MkEq(word, second))
			if err != nil {
				return err
			}
			equality, err = pin(m.MkOr(left, right))
			if err != nil {
				return err
			}
		}
		leftRegex, err := m.GetArg(regex, 0)
		if err != nil {
			return err
		}
		rightRegex, err := m.GetArg(regex, 1)
		if err != nil {
			return err
		}
		leftMember, err := pin(t.regexMembership(first, leftRegex))
		if err != nil {
			return err
		}
		rightMember, err := pin(t.regexMembership(second, rightRegex))
		if err != nil {
			return err
		}
		equivalent, err := pin(m.MkEq(expression, equality))
		if err != nil {
			return err
		}
		axiom, err = pin(m.MkAndArray([]ast.ASTHandle{leftMember, rightMember, equivalent}))
		if err != nil {
			return err
		}
	case 14:
		child, err := m.GetArg(regex, 0)
		if err != nil {
			return err
		}
		bound, err := t.freshUnrollBound()
		if err != nil {
			return err
		}
		unroll, err := pin(m.MkFamilyApp(t.FamilyID(), 43, nil, []ast.ASTHandle{child, bound}))
		if err != nil {
			return err
		}
		if err := t.retain(unroll); err != nil {
			return err
		}
		zero, err := pin(m.ArithmeticIntegerNumeralText("0"))
		if err != nil {
			return err
		}
		empty, err := pin(t.stringLiteral(""))
		if err != nil {
			return err
		}
		boundZero, err := pin(m.MkEq(bound, zero))
		if err != nil {
			return err
		}
		unrollEmpty, err := pin(m.MkEq(unroll, empty))
		if err != nil {
			return err
		}
		zeroIffEmpty, err := pin(m.MkEq(boundZero, unrollEmpty))
		if err != nil {
			return err
		}
		nonnegativeBound, err := pin(m.MkArithmeticGE(bound, zero))
		if err != nil {
			return err
		}
		length, err := pin(m.MkSequenceLength(unroll))
		if err != nil {
			return err
		}
		nonnegativeLength, err := pin(m.MkArithmeticGE(length, zero))
		if err != nil {
			return err
		}
		definition, err := pin(m.MkAndArray([]ast.ASTHandle{zeroIffEmpty, nonnegativeBound, nonnegativeLength}))
		if err != nil {
			return err
		}
		if err := t.assertAxiom(definition); err != nil {
			return err
		}
		// instantiate_axiom_RegexIn requests the empty literal again after
		// mk_unroll returns. Preserve that observable string-cache access.
		emptyAgain, err := pin(t.stringLiteral(""))
		if err != nil {
			return err
		}
		outerEmpty, err := pin(m.MkEq(unroll, emptyAgain))
		if err != nil {
			return err
		}
		outerZeroIffEmpty, err := pin(m.MkEq(boundZero, outerEmpty))
		if err != nil {
			return err
		}
		wordEquality, err := pin(m.MkEq(word, unroll))
		if err != nil {
			return err
		}
		equivalent, err := pin(m.MkEq(expression, wordEquality))
		if err != nil {
			return err
		}
		axiom, err = pin(m.MkAndArray([]ast.ASTHandle{equivalent, outerZeroIffEmpty}))
		if err != nil {
			return err
		}
	case 16:
		lo, err := m.GetArg(regex, 0)
		if err != nil {
			return err
		}
		hi, err := m.GetArg(regex, 1)
		if err != nil {
			return err
		}
		lower, lk, err := m.SequenceStringValue(lo)
		if err != nil {
			return err
		}
		upper, uk, err := m.SequenceStringValue(hi)
		if err != nil {
			return err
		}
		if !lk || !uk || len(lower) != 1 || len(upper) != 1 {
			return ast.ErrASTDomain
		}
		first, last := int(lower[0]), int(upper[0])
		if first > last {
			first, last = last, first
		}
		var cases []ast.ASTHandle
		for ch := first; ch <= last; ch++ {
			literal, err := pin(m.MkString(string([]byte{byte(ch)})))
			if err != nil {
				return err
			}
			equality, err := pin(m.MkEq(word, literal))
			if err != nil {
				return err
			}
			cases = append(cases, equality)
		}
		disjunction, err := pin(m.MkOrArray(cases))
		if err != nil {
			return err
		}
		axiom, err = pin(m.MkIff(expression, disjunction))
		if err != nil {
			return err
		}
	case 23:
		axiom = expression
	case 24:
		length, err := pin(m.MkSequenceLength(word))
		if err != nil {
			return err
		}
		one, err := pin(m.ArithmeticIntegerNumeralText("1"))
		if err != nil {
			return err
		}
		equality, err := pin(m.MkEq(length, one))
		if err != nil {
			return err
		}
		axiom, err = pin(m.MkIff(expression, equality))
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("string theory legacy regex operation %d is not supported", op)
	}
	return t.assertAxiom(axiom)
}
