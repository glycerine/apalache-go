package smtcore

// Source: src/smt/theory_str.cpp regex_inc_counter/regex_get_counter,
// _qadd/_qmul, estimate_regex_complexity/under_complement and
// estimate_automata_intersection_difficulty.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/automata"
)

func stringRegexCounter(counter map[ast.ASTHandle]uint32, key ast.ASTHandle) uint32 {
	value, found := counter[key]
	if !found {
		counter[key] = 0 // Native reads insert a missing key.
	}
	return value
}

func incrementStringRegexCounter(counter map[ast.ASTHandle]uint32, key ast.ASTHandle) {
	counter[key]++ // Native unsigned wrap is intentional.
}

func stringRegexComplexityAdd(left, right uint32) uint32 {
	if left == ^uint32(0) || right == ^uint32(0) {
		return ^uint32(0)
	}
	result := left + right
	if result < left || result < right {
		return ^uint32(0)
	}
	return result
}

func stringRegexComplexityMultiply(left, right uint32) uint32 {
	if left == ^uint32(0) || right == ^uint32(0) {
		return ^uint32(0) // Native sentinel check precedes the zero check.
	}
	if left == 0 || right == 0 {
		return 0
	}
	result := left * right
	// Preserve this source comparison-based test, including products whose
	// wrapped result is not smaller than either operand. Do not replace it
	// with a different mathematical overflow detector.
	if result < left || result < right {
		return ^uint32(0)
	}
	return result
}

func (t *SMTStringTheory) estimateStringRegexComplexity(regex ast.ASTHandle, underComplement bool) (uint32, error) {
	m := t.state.manager
	sort, err := m.GetSort(regex)
	if err != nil {
		return 0, err
	}
	if _, matched, err := m.RegexSequenceSort(sort); err != nil || !matched {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return 0, err
	}
	op, own, err := t.operation(regex)
	if err != nil || !own {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return 0, err
	}
	switch op {
	case 11:
		word, err := m.GetArg(regex, 0)
		if err != nil {
			return 0, err
		}
		text, constant, err := m.SequenceStringValue(word)
		if err != nil || !constant {
			if err == nil {
				err = ast.ErrASTDomain
			}
			return 0, err
		}
		return uint32(len(text)), nil
	case 21:
		child, err := m.GetArg(regex, 0)
		if err != nil {
			return 0, err
		}
		return t.estimateStringRegexComplexity(child, true)
	case 17, 18:
		left, err := m.GetArg(regex, 0)
		if err != nil {
			return 0, err
		}
		right, err := m.GetArg(regex, 1)
		if err != nil {
			return 0, err
		}
		first, err := t.estimateStringRegexComplexity(left, underComplement)
		if err != nil {
			return 0, err
		}
		second, err := t.estimateStringRegexComplexity(right, underComplement)
		if err != nil {
			return 0, err
		}
		if underComplement {
			if op == 18 {
				return stringRegexComplexityMultiply(first, second), nil
			}
			return stringRegexComplexityAdd(stringRegexComplexityMultiply(2, first), second), nil
		}
		return stringRegexComplexityAdd(first, second), nil
	case 13, 14:
		child, err := m.GetArg(regex, 0)
		if err != nil {
			return 0, err
		}
		value, err := t.estimateStringRegexComplexity(child, underComplement)
		if err != nil {
			return 0, err
		}
		return stringRegexComplexityMultiply(2, value), nil
	case 16:
		if err := t.regexLengthRangePreconditions(regex); err != nil {
			return 0, err
		}
		left, err := m.GetArg(regex, 0)
		if err != nil {
			return 0, err
		}
		right, err := m.GetArg(regex, 1)
		if err != nil {
			return 0, err
		}
		lower, _, err := m.SequenceStringValue(left)
		if err != nil {
			return 0, err
		}
		upper, _, err := m.SequenceStringValue(right)
		if err != nil {
			return 0, err
		}
		return 1 + uint32(upper[0]) - uint32(lower[0]), nil
	default:
		return 1, nil // Includes full-char/full-sequence and unknown operations.
	}
}

func estimateStringAutomataIntersectionDifficulty(left, right *automata.Automaton) (uint32, error) {
	if left == nil || right == nil {
		return 0, ast.ErrASTDomain
	}
	return stringRegexComplexityMultiply(left.NumStates(), right.NumStates()), nil
}
