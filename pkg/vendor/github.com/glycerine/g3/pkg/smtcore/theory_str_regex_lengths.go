package smtcore

// Source: src/smt/theory_str.cpp check_regex_length_linearity,
// check_regex_length_linearity_helper, check_subterm_lengths and
// infer_all_regex_lengths. The enclosing automata stage remains separate.

import (
	"errors"
	"sort"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) checkRegexLengthLinearity(regex ast.ASTHandle, insideStar bool) (bool, error) {
	op, own, err := t.operation(regex)
	if err != nil || !own {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return false, err
	}
	m := t.state.manager
	switch op {
	case 11, 16, 23, 24:
		return true, nil
	case 17, 18:
		left, err := m.GetArg(regex, 0)
		if err != nil {
			return false, err
		}
		first, err := t.checkRegexLengthLinearity(left, insideStar)
		if err != nil || !first {
			return first, err
		}
		right, err := m.GetArg(regex, 1)
		if err != nil {
			return false, err
		}
		return t.checkRegexLengthLinearity(right, insideStar)
	case 13, 14:
		if insideStar {
			return false, nil
		}
		child, err := m.GetArg(regex, 0)
		if err != nil {
			return false, err
		}
		return t.checkRegexLengthLinearity(child, true)
	case 21:
		return false, nil
	default:
		return false, ast.ErrASTDomain // Native UNREACHABLE for unknown operations.
	}
}

func (t *SMTStringTheory) finiteRegexSubtermLengths(regex ast.ASTHandle) (map[int32]struct{}, error) {
	lens := make(map[int32]struct{})
	m := t.state.manager
	op, own, err := t.operation(regex)
	if err != nil || !own {
		return lens, err
	}
	switch op {
	case 11:
		word, err := m.GetArg(regex, 0)
		if err != nil {
			return lens, err
		}
		text, constant, err := m.SequenceStringValue(word)
		if err != nil || !constant {
			if err == nil {
				err = ast.ErrASTDomain
			}
			return lens, err
		}
		lens[int32(len(text))] = struct{}{}
	case 17, 18:
		left, err := m.GetArg(regex, 0)
		if err != nil {
			return lens, err
		}
		right, err := m.GetArg(regex, 1)
		if err != nil {
			return lens, err
		}
		first, err := t.finiteRegexSubtermLengths(left)
		if err != nil {
			return lens, err
		}
		second, err := t.finiteRegexSubtermLengths(right)
		if err != nil || len(first) == 0 || len(second) == 0 {
			return lens, err // Empty means inference failed, including for union.
		}
		if op == 17 {
			for a := range first {
				for b := range second {
					lens[a+b] = struct{}{}
				}
			}
		} else {
			for value := range first {
				lens[value] = struct{}{}
			}
			for value := range second {
				lens[value] = struct{}{}
			}
		}
	case 16:
		if err := t.regexLengthRangePreconditions(regex); err != nil {
			return lens, err
		}
		lens[1] = struct{}{}
	case 24:
		lens[1] = struct{}{}
		// Star/plus, full sequence, complement and unknown operations all yield
		// an empty set here, unlike the standalone linearity/formula routines.
	}
	return lens, nil
}

func (t *SMTStringTheory) regexLengthRangePreconditions(regex ast.ASTHandle) error {
	for i := uint32(0); i < 2; i++ {
		argument, err := t.state.manager.GetArg(regex, i)
		if err != nil {
			return err
		}
		text, constant, err := t.state.manager.SequenceStringValue(argument)
		if err != nil {
			return err
		}
		if !constant || len(text) != 1 {
			return ast.ErrASTDomain
		}
	}
	return nil
}

type smtStringRegexLengthBuilder struct {
	theory        *SMTStringTheory
	freeVariables *[]ast.ASTHandle
	locals        smtStringLengthPremises
}

func (b *smtStringRegexLengthBuilder) pin(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return value, err
	}
	return value, b.locals.add(value)
}

func (t *SMTStringTheory) inferStringRegexLengths(length, regex ast.ASTHandle, freeVariables *[]ast.ASTHandle) (root ast.ASTRef, result error) {
	if freeVariables == nil {
		return root, ast.ErrASTDomain
	}
	m := t.state.manager
	sort, err := m.GetSort(regex)
	if err != nil {
		return root, err
	}
	family, err := m.GetFamilyID(sort)
	if err != nil {
		return root, err
	}
	kind, err := m.GetDeclKind(sort)
	if err != nil {
		return root, err
	}
	if family != t.FamilyID() || kind != 1 {
		return root, ast.ErrASTDomain
	}
	b := smtStringRegexLengthBuilder{t, freeVariables, smtStringLengthPremises{theory: t}}
	defer func() { result = errors.Join(result, b.locals.close()) }()
	value, err := b.build(length, regex)
	if err != nil {
		return root, err
	}
	return m.NewRef(value)
}

func (b *smtStringRegexLengthBuilder) fresh(name string) (ast.ASTHandle, error) {
	variable, err := b.theory.freshIntegerVariable(name)
	if err == nil {
		*b.freeVariables = append(*b.freeVariables, variable)
	}
	return variable, err
}

func (b *smtStringRegexLengthBuilder) build(length, regex ast.ASTHandle) (ast.ASTHandle, error) {
	t, m := b.theory, b.theory.state.manager
	op, own, err := t.operation(regex)
	if err != nil || !own {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, err
	}
	switch op {
	case 11:
		word, err := m.GetArg(regex, 0)
		if err != nil {
			return word, err
		}
		text, constant, err := m.SequenceStringValue(word)
		if err != nil || !constant {
			if err == nil {
				err = ast.ErrASTDomain
			}
			return ast.ASTHandle{}, err
		}
		numeral, err := b.pin(m.ArithmeticIntegerNumeralText(strconv.Itoa(len(text))))
		if err != nil {
			return numeral, err
		}
		return b.pin(m.MkEq(length, numeral))
	case 17, 18:
		left, err := m.GetArg(regex, 0)
		if err != nil {
			return left, err
		}
		right, err := m.GetArg(regex, 1)
		if err != nil {
			return right, err
		}
		firstLength, secondLength := length, length
		if op == 17 {
			firstLength, err = b.fresh("rlen1")
			if err != nil {
				return firstLength, err
			}
			secondLength, err = b.fresh("rlen2")
			if err != nil {
				return secondLength, err
			}
		}
		first, err := b.build(firstLength, left)
		if err != nil {
			return first, err
		}
		second, err := b.build(secondLength, right)
		if err != nil {
			return second, err
		}
		if op == 18 {
			return b.pin(m.MkOr(first, second))
		}
		sum, err := b.pin(m.MkArithmeticAdd(firstLength, secondLength))
		if err != nil {
			return sum, err
		}
		equality, err := b.pin(m.MkEq(length, sum))
		if err != nil {
			return equality, err
		}
		return b.pin(m.MkAndArray([]ast.ASTHandle{equality, first, second}))
	case 13, 14:
		child, err := m.GetArg(regex, 0)
		if err != nil {
			return child, err
		}
		lengths, err := t.finiteRegexSubtermLengths(child)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if len(lengths) != 0 {
			var options []int32
			for option := range lengths {
				options = append(options, option)
			}
			sort.Slice(options, func(i, j int) bool { return options[i] < options[j] })
			var terms []ast.ASTHandle
			for _, option := range options {
				variable, err := b.fresh("rstar")
				if err != nil {
					return variable, err
				}
				coefficient, err := b.pin(m.ArithmeticIntegerNumeralText(strconv.FormatInt(int64(option), 10)))
				if err != nil {
					return coefficient, err
				}
				term, err := b.pin(m.MkArithmeticMul(coefficient, variable))
				if err != nil {
					return term, err
				}
				if op == 13 {
					// Source adds a base contribution for every distinct length.
					term, err = b.pin(m.MkArithmeticAdd(coefficient, term))
					if err != nil {
						return term, err
					}
				}
				terms = append(terms, term)
			}
			sum := terms[0] // Native mk_add_simplify's single-item case.
			if len(terms) > 1 {
				family, err := m.ArithmeticFamilyID()
				if err != nil {
					return sum, err
				}
				sum, err = b.pin(m.MkFamilyApp(family, 6, nil, terms))
				if err != nil {
					return sum, err
				}
			}
			return b.pin(m.MkEq(length, sum))
		}
	case 16, 24:
		if op == 16 {
			if err := t.regexLengthRangePreconditions(regex); err != nil {
				return ast.ASTHandle{}, err
			}
		}
		one, err := b.pin(m.ArithmeticIntegerNumeralText("1"))
		if err != nil {
			return one, err
		}
		return b.pin(m.MkEq(length, one))
	}
	// Full sequence, complement, unknown operations and impossible finite
	// subterm inference share the native nonnegative-only fallback.
	zero, err := b.pin(m.ArithmeticIntegerNumeralText("0"))
	if err != nil {
		return zero, err
	}
	return b.pin(m.MkArithmeticGE(length, zero))
}
