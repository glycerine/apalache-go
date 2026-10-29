package smtcore

// Source: src/smt/theory_str.cpp mk_unroll and reduce_virtual_regex_in.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) makeStringUnroll(regex, bound ast.ASTHandle) (output ast.ASTHandle, result error) {
	m := t.state.manager
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	term, err := m.MkFamilyApp(t.FamilyID(), 43, nil, []ast.ASTHandle{regex, bound})
	if err != nil {
		return output, err
	}
	if err = t.retain(term); err != nil {
		return output, err
	}
	zero, err := pin(m.ArithmeticIntegerNumeralText("0"))
	if err != nil {
		return output, err
	}
	boundZero, err := pin(t.state.mkEqualityAtom(bound, zero))
	if err != nil {
		return output, err
	}
	empty, err := pin(t.stringLiteral(""))
	if err != nil {
		return output, err
	}
	termEmpty, err := pin(t.state.mkEqualityAtom(term, empty))
	if err != nil {
		return output, err
	}
	equivalence, err := pin(t.state.mkEqualityAtom(boundZero, termEmpty))
	if err != nil {
		return output, err
	}
	nonnegativeBound, err := pin(m.MkArithmeticGE(bound, zero))
	if err != nil {
		return output, err
	}
	length, err := pin(m.MkSequenceLength(term))
	if err != nil {
		return output, err
	}
	nonnegativeLength, err := pin(m.MkArithmeticGE(length, zero))
	if err != nil {
		return output, err
	}
	formula, err := pin(m.MkAndArray([]ast.ASTHandle{equivalence, nonnegativeBound, nonnegativeLength}))
	if err != nil {
		return output, err
	}
	if err = t.assertAxiom(formula); err != nil {
		return output, err
	}
	return term, nil
}

// items owns only emitted premises. Temporary operands belong to each
// recursive frame; range constraints go directly through assertAxiom.
func (t *SMTStringTheory) reduceStringVirtualRegexIn(word, regex ast.ASTHandle, items *smtStringLengthPremises) (result error) {
	m := t.state.manager
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	appendEq := func(left, right ast.ASTHandle) error {
		equal, err := t.state.mkEqualityAtom(left, right)
		if err != nil {
			return err
		}
		return items.add(equal)
	}
	appendLengths := func(left, right ast.ASTHandle) error {
		l, err := pin(m.MkSequenceLength(left))
		if err != nil {
			return err
		}
		r, err := pin(m.MkSequenceLength(right))
		if err != nil {
			return err
		}
		return appendEq(l, r)
	}
	op, own, err := t.operation(regex)
	if err != nil {
		return err
	}
	if !own {
		return ast.ErrASTDomain
	}
	switch op {
	case 11:
		inside, err := m.GetArg(regex, 0)
		if err != nil {
			return err
		}
		if err = appendEq(word, inside); err != nil {
			return err
		}
		return appendLengths(word, inside)
	case 18, 17: // union or concatenation
		first, err := t.freshRegexVariable()
		if err != nil {
			return err
		}
		second, err := t.freshRegexVariable()
		if err != nil {
			return err
		}
		if op == 18 {
			leftChoice, err := pin(t.state.mkEqualityAtom(word, first))
			if err != nil {
				return err
			}
			rightChoice, err := pin(t.state.mkEqualityAtom(word, second))
			if err != nil {
				return err
			}
			choice, err := pin(m.MkOr(leftChoice, rightChoice))
			if err != nil {
				return err
			}
			if err = items.add(choice); err != nil {
				return err
			}
			for _, entry := range []struct{ choice, term ast.ASTHandle }{{leftChoice, first}, {rightChoice, second}} {
				l, err := pin(m.MkSequenceLength(word))
				if err != nil {
					return err
				}
				r, err := pin(m.MkSequenceLength(entry.term))
				if err != nil {
					return err
				}
				equal, err := pin(t.state.mkEqualityAtom(l, r))
				if err != nil {
					return err
				}
				negative, err := pin(m.MkNot(entry.choice))
				if err != nil {
					return err
				}
				guarded, err := pin(m.MkOr(negative, equal))
				if err != nil {
					return err
				}
				if err = items.add(guarded); err != nil {
					return err
				}
			}
		} else {
			concat, err := pin(t.concatTerm(first, second))
			if err != nil {
				return err
			}
			if err = appendEq(word, concat); err != nil {
				return err
			}
			l, err := pin(m.MkSequenceLength(first))
			if err != nil {
				return err
			}
			r, err := pin(m.MkSequenceLength(second))
			if err != nil {
				return err
			}
			sum, err := pin(m.MkArithmeticAdd(l, r))
			if err != nil {
				return err
			}
			length, err := pin(m.MkSequenceLength(word))
			if err != nil {
				return err
			}
			if err = appendEq(length, sum); err != nil {
				return err
			}
		}
		left, err := m.GetArg(regex, 0)
		if err != nil {
			return err
		}
		if err = t.reduceStringVirtualRegexIn(first, left, items); err != nil {
			return err
		}
		right, err := m.GetArg(regex, 1)
		if err != nil {
			return err
		}
		return t.reduceStringVirtualRegexIn(second, right, items)
	case 14:
		child, err := m.GetArg(regex, 0)
		if err != nil {
			return err
		}
		bound, err := t.freshUnrollBound()
		if err != nil {
			return err
		}
		term, err := pin(t.makeStringUnroll(child, bound))
		if err != nil {
			return err
		}
		if err = appendEq(word, term); err != nil {
			return err
		}
		return appendLengths(word, term)
	case 16:
		lo, err := m.GetArg(regex, 0)
		if err != nil {
			return err
		}
		hi, err := m.GetArg(regex, 1)
		if err != nil {
			return err
		}
		low, literal, err := m.SequenceStringValue(lo)
		if err != nil {
			return err
		}
		if !literal || len(low) != 1 {
			return ast.ErrASTDomain
		}
		high, literal, err := m.SequenceStringValue(hi)
		if err != nil {
			return err
		}
		if !literal || len(high) != 1 {
			return ast.ErrASTDomain
		}
		first, last := uint32(low[0]), uint32(high[0])
		if first > last {
			first, last = last, first
		}
		var cases []ast.ASTHandle
		for ch := first; ch <= last; ch++ {
			// Native u.str.mk_string(zstring(unsigned ch)) preserves NUL and
			// bypasses theory_str::mk_string and its String literal cache.
			literal, err := pin(m.MkString(string([]byte{byte(ch)})))
			if err != nil {
				return err
			}
			equal, err := pin(t.state.mkEqualityAtom(word, literal))
			if err != nil {
				return err
			}
			cases = append(cases, equal)
		}
		formula, err := pin(m.MkOrArray(cases))
		if err != nil {
			return err
		}
		return t.assertAxiom(formula)
	default:
		return ast.ErrASTDomain // Native raises unrecognized regex operator.
	}
}
