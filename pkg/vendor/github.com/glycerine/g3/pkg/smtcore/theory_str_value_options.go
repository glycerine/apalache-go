package smtcore

// Source: src/smt/theory_str.cpp gen_val_options.

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) generateStringValueOptions(variable, lengthIndicator, valueIndicator ast.ASTHandle, lengthText string, tries int32) (output ast.ASTRef, result error) {
	// Retained length selectors produce nonnegative decimal integers. Native
	// atoi and signed window multiplication are undefined on overflow.
	parsed, err := strconv.ParseInt(lengthText, 10, 32)
	if err != nil || parsed < 0 || parsed == 1<<31-1 || tries < 0 || int64(tries)*32 > 1<<31-1 {
		return output, ast.ErrASTDomain
	}
	length := int32(parsed)
	var base []int32
	covered := false
	previous := t.testers.valueIndicators[variable][length]
	if tries == 0 {
		base = make([]int32, int(length)+1)
	} else {
		if int64(len(previous)) < int64(tries) {
			return output, ast.ErrASTDomain
		}
		base, covered, err = t.nextStringValueEncoding(t.testers.valueRanges[previous[tries-1].variable])
		if err != nil {
			return output, err
		}
	}
	var encodings [][]int32
	for i := 0; i < 32 && !covered; i++ {
		encodings = append(encodings, base)
		base, covered, err = t.nextStringValueEncoding(base)
		if err != nil {
			return output, err
		}
	}
	// Reentering after complete coverage would index an empty native vector.
	if len(encodings) == 0 {
		return output, ast.ErrASTDomain
	}
	t.testers.valueRanges[valueIndicator] = append([]int32(nil), encodings[len(encodings)-1]...)
	m := t.state.manager
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(expression ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return expression, err
		}
		return expression, locals.add(expression)
	}
	options := SMTStrState{}
	if t.parameterOwner != nil {
		options, err = t.stringParameterState()
		if err != nil {
			return output, err
		}
	}
	var choices, conjuncts []ast.ASTHandle
	for index, encoding := range encodings {
		ordinal := int64(tries)*32 + int64(index)
		label, err := pin(t.stringLiteral(strconv.FormatInt(ordinal, 10)))
		if err != nil {
			return output, err
		}
		choice, err := pin(m.MkEq(valueIndicator, label))
		if err != nil {
			return output, err
		}
		choices = append(choices, choice)
		if options.AggressiveValueTesting {
			if err = t.forceStringTesterPhase(valueIndicator, label, true); err != nil {
				return output, err
			}
		}
		text, err := t.stringValueFromEncoding(length, encoding)
		if err != nil {
			return output, err
		}
		var word ast.ASTHandle
		if options.UseFastValueTesterCache {
			var found bool
			word, found = t.testers.valueLiterals[text]
			if !found {
				word, err = pin(t.stringLiteral(text))
				if err != nil {
					return output, err
				}
				t.testers.valueLiterals[text] = word
				if err = t.retain(word); err != nil {
					return output, err
				}
			} else if err = locals.add(word); err != nil {
				return output, err
			}
		} else {
			word, err = pin(t.stringLiteral(text))
			if err != nil {
				return output, err
			}
		}
		equal, err := pin(m.MkEq(variable, word))
		if err != nil {
			return output, err
		}
		equivalence, err := pin(m.MkEq(choice, equal))
		if err != nil {
			return output, err
		}
		conjuncts = append(conjuncts, equivalence)
	}
	if !covered {
		more, err := pin(t.stringLiteral("more"))
		if err != nil {
			return output, err
		}
		choice, err := pin(m.MkEq(valueIndicator, more))
		if err != nil {
			return output, err
		}
		choices = append(choices, choice)
		if options.AggressiveValueTesting {
			if err = t.forceStringTesterPhase(valueIndicator, more, false); err != nil {
				return output, err
			}
		}
	}
	disjunction, err := pin(m.MkOrArray(choices))
	if err != nil {
		return output, err
	}
	conjuncts = append(conjuncts, disjunction)
	formula, err := pin(m.MkAndArray(conjuncts))
	if err != nil {
		return output, err
	}
	lengthLabel, err := pin(t.stringLiteral(lengthText))
	if err != nil {
		return output, err
	}
	lengthChoice, err := pin(m.MkEq(lengthIndicator, lengthLabel))
	if err != nil {
		return output, err
	}
	premises := []ast.ASTHandle{lengthChoice}
	for index := int32(0); index < tries; index++ {
		prior := previous[index].variable
		if prior == valueIndicator {
			continue
		}
		more, err := pin(t.stringLiteral("more"))
		if err != nil {
			return output, err
		}
		condition, err := pin(m.MkEq(prior, more))
		if err != nil {
			return output, err
		}
		premises = append(premises, condition)
	}
	guard, err := pin(m.MkAndArray(premises))
	if err != nil {
		return output, err
	}
	negative, err := pin(m.MkNot(guard))
	if err != nil {
		return output, err
	}
	formula, err = pin(m.MkOr(negative, formula))
	if err != nil {
		return output, err
	}
	return m.NewRef(formula)
}
