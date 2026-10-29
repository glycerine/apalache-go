package smtcore

// Source: src/smt/theory_str.cpp binary_search_case_split,
// binary_search_length_test and gen_len_val_options_for_free_var dispatch;
// theory_str.h binary_search_info and binary_search_trail.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// All retained bounds start integral and remain integral. Each stored window
// owns its integers and is immutable after publication.
type smtStringBinaryBounds struct{ lower, midpoint, upper, window *big.Int }
type smtStringBinaryTrail struct {
	level        uint32
	word, tester ast.ASTHandle
}

func (b *smtStringBinaryBounds) calculateMidpoint() {
	b.midpoint = new(big.Int).Div(new(big.Int).Add(b.lower, b.upper), big.NewInt(2))
}

func (t *SMTStringTheory) recordStringBinaryTest(word, tester ast.ASTHandle, bounds *smtStringBinaryBounds) {
	t.testers.binaryStacks[word] = append(t.testers.binaryStacks[word], tester)
	t.testers.binaryBounds[tester] = bounds
	t.testers.binaryTrail = append(t.testers.binaryTrail, smtStringBinaryTrail{level: t.level, word: word, tester: tester})
}

func (t *SMTStringTheory) popStringBinaryTests() {
	for len(t.testers.binaryTrail) > 0 {
		last := len(t.testers.binaryTrail) - 1
		entry := t.testers.binaryTrail[last]
		if entry.level <= t.level {
			break
		}
		// insert_obj_map::undo erases; it does not restore an old value.
		delete(t.testers.binaryBounds, entry.tester)
		stack := t.testers.binaryStacks[entry.word]
		if len(stack) > 0 {
			t.testers.binaryStacks[entry.word] = stack[:len(stack)-1]
		}
		t.testers.binaryTrail[last] = smtStringBinaryTrail{}
		t.testers.binaryTrail = t.testers.binaryTrail[:last]
	}
}

func (t *SMTStringTheory) generateStringBinaryCaseSplit(word, tester ast.ASTHandle, bounds *smtStringBinaryBounds) (output ast.ASTRef, result error) {
	if bounds == nil || bounds.midpoint == nil {
		return output, ast.ErrASTDomain
	}
	m := t.state.manager
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	length, err := pin(m.MkSequenceLength(word))
	if err != nil {
		return output, err
	}
	var choices, conjuncts []ast.ASTHandle
	for _, label := range []string{"less", "more", bounds.midpoint.String()} {
		text, err := pin(t.stringLiteral(label))
		if err != nil {
			return output, err
		}
		choice, err := pin(t.state.mkEqualityAtom(tester, text))
		if err != nil {
			return output, err
		}
		choices = append(choices, choice)
		value := new(big.Int).Set(bounds.midpoint)
		if label == "less" {
			value.Sub(value, big.NewInt(1))
		} else if label == "more" {
			value.Add(value, big.NewInt(1))
		}
		number, err := pin(m.ArithmeticIntegerNumeralText(value.String()))
		if err != nil {
			return output, err
		}
		var relation ast.ASTHandle
		switch label {
		case "less":
			relation, err = pin(m.MkArithmeticLE(length, number))
		case "more":
			relation, err = pin(m.MkArithmeticGE(length, number))
		default:
			relation, err = pin(t.state.mkEqualityAtom(length, number))
		}
		if err != nil {
			return output, err
		}
		equivalence, err := pin(t.state.mkEqualityAtom(choice, relation))
		if err != nil {
			return output, err
		}
		conjuncts = append(conjuncts, equivalence)
	}
	disjunction, err := pin(m.MkOrArray(choices))
	if err != nil {
		return output, err
	}
	conjuncts = append(conjuncts, disjunction)
	// The native case_split_literals vector is unused, but its explicit
	// Boolean internalizations and literal lookups remain live side effects.
	for _, choice := range choices {
		if _, exists := t.state.BoolVar(choice); !exists {
			if err = t.state.Internalize(choice, false); err != nil {
				return output, err
			}
		}
		if _, err = t.state.Literal(choice); err != nil {
			return output, err
		}
	}
	formula, err := pin(m.MkAndArray(conjuncts))
	if err != nil {
		return output, err
	}
	return m.NewRef(formula)
}

func (t *SMTStringTheory) generateStringBinaryLengthOptions(word, callback ast.ASTHandle, callbackValue string) (output ast.ASTRef, result error) {
	m := t.state.manager
	stack := t.testers.binaryStacks[word]
	var tester ast.ASTHandle
	var bounds *smtStringBinaryBounds
	if len(stack) > 0 {
		last := stack[len(stack)-1]
		value, assigned, err := t.eqcStringValue(last)
		if err != nil {
			return output, err
		}
		var text string
		if !assigned {
			if callback != last {
				return output, ast.ErrASTDomain
			}
			text = callbackValue
		} else {
			text, _, err = m.SequenceStringValue(value)
			if err != nil {
				return output, err
			}
		}
		previous := t.testers.binaryBounds[last]
		if previous == nil {
			return output, ast.ErrASTDomain
		}
		if text != "more" && text != "less" {
			if previous.midpoint.Sign() < 0 {
				locals := smtStringLengthPremises{theory: t}
				defer func() { result = errors.Join(result, locals.close()) }()
				length, err := m.MkSequenceLength(word)
				if err != nil {
					return output, err
				}
				if err = locals.add(length); err != nil {
					return output, err
				}
				zero, err := m.ArithmeticIntegerNumeralText("0")
				if err != nil {
					return output, err
				}
				formula, err := m.MkArithmeticGE(length, zero)
				if err != nil {
					return output, err
				}
				return m.NewRef(formula)
			}
			return t.generateStringFreeValueOptions(word, last, text, ast.ASTHandle{}, "")
		}
		bounds = &smtStringBinaryBounds{lower: new(big.Int).Set(previous.lower), upper: new(big.Int).Set(previous.upper), window: new(big.Int).Set(previous.window)}
		cache := t.testers.binaryLow
		if text == "more" {
			cache = t.testers.binaryHigh
			if previous.midpoint.Cmp(previous.upper) == 0 && previous.upper.Cmp(previous.window) == 0 {
				bounds.lower.Add(previous.window, big.NewInt(1))
				bounds.window.Mul(previous.window, big.NewInt(2))
				bounds.upper.Set(bounds.window)
			} else {
				bounds.lower.Add(previous.midpoint, big.NewInt(1))
			}
		} else {
			bounds.upper.Sub(previous.midpoint, big.NewInt(1))
		}
		bounds.calculateMidpoint()
		tester = cache[last]
		if tester.IsNull() {
			if !bounds.midpoint.IsInt64() || bounds.midpoint.Int64() < -1<<31 || bounds.midpoint.Int64() > 1<<31-1 {
				return output, ast.ErrASTDomain
			}
			tester, err = t.freshStringLengthTester(word, int32(bounds.midpoint.Int64()))
			if err != nil {
				return output, err
			}
			cache[last] = tester
		}
		if err = t.refreshStringVariable(tester); err != nil {
			return output, err
		}
	} else {
		t.testers.binaryStacks[word] = nil
		upper := uint32(64)
		if t.parameterOwner != nil {
			options, err := t.stringParameterState()
			if err != nil {
				return output, err
			}
			upper = options.BinarySearchInitialUpperBound
		}
		bounds = &smtStringBinaryBounds{lower: new(big.Int), upper: new(big.Int).SetUint64(uint64(upper)), window: new(big.Int).SetUint64(uint64(upper))}
		bounds.calculateMidpoint()
		tester = t.testers.binaryStart[word]
		if tester.IsNull() {
			// A uint32 initial bound has an int32-representable midpoint.
			var err error
			tester, err = t.freshStringLengthTester(word, int32(bounds.midpoint.Int64()))
			if err != nil {
				return output, err
			}
			t.testers.binaryStart[word] = tester
		}
		if err := t.refreshStringVariable(tester); err != nil {
			return output, err
		}
		known, ok, err := t.lengthValue(word)
		if err != nil {
			return output, err
		}
		if ok {
			bounds.midpoint = new(big.Int).Set(known)
			bounds.upper = new(big.Int).Mul(known, big.NewInt(2))
			bounds.window = new(big.Int).Set(bounds.upper)
		}
	}
	t.recordStringBinaryTest(word, tester, bounds)
	root, err := t.generateStringBinaryCaseSplit(word, tester, bounds)
	if err != nil {
		return output, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&root)) }()
	formula, err := m.RefGet(root)
	if err != nil {
		return output, err
	}
	if err = t.retain(formula); err != nil {
		return output, err
	}
	return m.NewRef(formula)
}

func (t *SMTStringTheory) generateStringLengthValueOptions(word, callback ast.ASTHandle, callbackValue string) (ast.ASTRef, error) {
	if t.parameterOwner != nil {
		options, err := t.stringParameterState()
		if err != nil {
			return ast.ASTRef{}, err
		}
		if options.UseBinarySearch {
			return t.generateStringBinaryLengthOptions(word, callback, callbackValue)
		}
	}
	return t.generateStringSequentialOptions(word, callback, callbackValue)
}
