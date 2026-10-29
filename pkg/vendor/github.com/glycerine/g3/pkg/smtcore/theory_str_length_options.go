package smtcore

// Source: src/smt/theory_str.cpp gen_len_test_options.

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

type smtStringTesterState struct {
	lengthID, valueID       int32
	lengthIndicators        map[ast.ASTHandle][]ast.ASTHandle
	lengthLiterals          map[int32]ast.ASTHandle
	lengthCounts            map[ast.ASTHandle]uint32
	lengthSubjects          map[ast.ASTHandle]ast.ASTHandle
	valueIndicators         map[ast.ASTHandle]map[int32][]smtStringScopedIndicator
	valueRanges             map[ast.ASTHandle][]int32
	valueLiterals           map[string]ast.ASTHandle
	valueSubjects           map[ast.ASTHandle]ast.ASTHandle
	binaryStacks            map[ast.ASTHandle][]ast.ASTHandle
	binaryBounds            map[ast.ASTHandle]*smtStringBinaryBounds
	binaryStart             map[ast.ASTHandle]ast.ASTHandle
	binaryHigh              map[ast.ASTHandle]ast.ASTHandle
	binaryLow               map[ast.ASTHandle]ast.ASTHandle
	binaryTrail             []smtStringBinaryTrail
	unrollHistories         map[ast.ASTHandle][]*smtStringUnrollHistory
	unrollDefinitions       map[ast.ASTHandle]ast.ASTHandle
	concatUnrollDefinitions map[[2]ast.ASTHandle]ast.ASTHandle
}

type smtStringScopedIndicator struct {
	level    uint32
	variable ast.ASTHandle
}

func newStringTesterState() smtStringTesterState {
	return smtStringTesterState{
		lengthIndicators: make(map[ast.ASTHandle][]ast.ASTHandle), lengthLiterals: make(map[int32]ast.ASTHandle),
		lengthCounts: make(map[ast.ASTHandle]uint32), lengthSubjects: make(map[ast.ASTHandle]ast.ASTHandle),
		valueIndicators: make(map[ast.ASTHandle]map[int32][]smtStringScopedIndicator),
		valueRanges:     make(map[ast.ASTHandle][]int32), valueLiterals: make(map[string]ast.ASTHandle),
		valueSubjects: make(map[ast.ASTHandle]ast.ASTHandle),
		binaryStacks:  make(map[ast.ASTHandle][]ast.ASTHandle), binaryBounds: make(map[ast.ASTHandle]*smtStringBinaryBounds),
		binaryStart: make(map[ast.ASTHandle]ast.ASTHandle), binaryHigh: make(map[ast.ASTHandle]ast.ASTHandle), binaryLow: make(map[ast.ASTHandle]ast.ASTHandle),
		unrollHistories:         make(map[ast.ASTHandle][]*smtStringUnrollHistory),
		unrollDefinitions:       make(map[ast.ASTHandle]ast.ASTHandle),
		concatUnrollDefinitions: make(map[[2]ast.ASTHandle]ast.ASTHandle),
	}
}

func (t *SMTStringTheory) forceStringTesterPhase(left, right ast.ASTHandle, phase bool) error {
	literal, err := t.MkEqualityLiteral(left, right, false)
	if err != nil {
		return err
	}
	expression, err := t.state.BoolExpression(literal.Var())
	if err != nil {
		return err
	}
	if err = t.state.markRelevant(expression); err != nil {
		return err
	}
	data := &t.state.booleanData[literal.Var()]
	// context::force_phase writes cached phase only; it does not assign the
	// literal or set the independent try-true-first flag.
	data.PhaseAvailable = true
	data.Phase = phase
	return nil
}

func (t *SMTStringTheory) generateStringLengthOptions(variable, indicator ast.ASTHandle, tries int32) (output ast.ASTRef, result error) {
	m := t.state.manager
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	length, err := pin(m.MkSequenceLength(variable))
	if err != nil {
		return output, err
	}
	value, known, err := t.lengthValue(variable)
	if err != nil {
		return output, err
	}
	if known {
		label, err := pin(t.stringLiteral(value.String()))
		if err != nil {
			return output, err
		}
		option, err := pin(t.state.mkEqualityAtom(indicator, label))
		if err != nil {
			return output, err
		}
		number, err := pin(m.ArithmeticIntegerNumeralText(value.String()))
		if err != nil {
			return output, err
		}
		exact, err := pin(t.state.mkEqualityAtom(length, number))
		if err != nil {
			return output, err
		}
		equivalence, err := pin(t.state.mkEqualityAtom(option, exact))
		if err != nil {
			return output, err
		}
		formula, err := pin(m.MkAnd(option, equivalence))
		if err != nil {
			return output, err
		}
		if err = t.retain(formula); err != nil {
			return output, err
		}
		return m.NewRef(formula)
	}
	// Retained sequential callers start at one. Beyond this range, native
	// signed-int multiplication has undefined overflow rather than a window.
	if tries <= 0 || int64(tries)*3 > int64(1<<31-1) {
		return output, ast.ErrASTDomain
	}
	options := SMTStrState{}
	if t.parameterOwner != nil {
		options, err = t.stringParameterState()
		if err != nil {
			return output, err
		}
	}
	lower, upper := (tries-1)*3, tries*3
	var choices, equivalences []ast.ASTHandle
	for index := lower; index < upper; index++ {
		var label ast.ASTHandle
		if options.UseFastLengthTesterCache {
			var cached bool
			label, cached = t.testers.lengthLiterals[index]
			if !cached {
				label, err = pin(t.stringLiteral(strconv.FormatInt(int64(index), 10)))
				if err != nil {
					return output, err
				}
				t.testers.lengthLiterals[index] = label
				if err = t.retain(label); err != nil {
					return output, err
				}
			} else if err = locals.add(label); err != nil {
				return output, err
			}
		} else {
			label, err = pin(t.stringLiteral(strconv.FormatInt(int64(index), 10)))
			if err != nil {
				return output, err
			}
		}
		choice, err := pin(t.state.mkEqualityAtom(indicator, label))
		if err != nil {
			return output, err
		}
		choices = append(choices, choice)
		priority := 0.2
		if index <= 5 {
			priority = 0.3
		}
		if err = t.addStringBranchPriority(choice, priority); err != nil {
			return output, err
		}
		if options.AggressiveLengthTesting {
			if err = t.forceStringTesterPhase(indicator, label, true); err != nil {
				return output, err
			}
		}
		number, err := pin(m.ArithmeticIntegerNumeralText(strconv.FormatInt(int64(index), 10)))
		if err != nil {
			return output, err
		}
		// The case_split_literals vector is never consumed outside disabled
		// code, but its mk_eq call still internalizes every arithmetic atom.
		if _, err = t.MkEqualityLiteral(length, number, false); err != nil {
			return output, err
		}
		exact, err := pin(m.MkEq(length, number))
		if err != nil {
			return output, err
		}
		equivalence, err := pin(t.state.mkEqualityAtom(choice, exact))
		if err != nil {
			return output, err
		}
		equivalences = append(equivalences, equivalence)
	}
	more, err := pin(t.stringLiteral("more"))
	if err != nil {
		return output, err
	}
	moreChoice, err := pin(t.state.mkEqualityAtom(indicator, more))
	if err != nil {
		return output, err
	}
	choices = append(choices, moreChoice)
	if err = t.addStringBranchPriority(moreChoice, -0.1); err != nil {
		return output, err
	}
	if options.AggressiveLengthTesting {
		if err = t.forceStringTesterPhase(indicator, more, false); err != nil {
			return output, err
		}
	}
	boundary, err := pin(m.ArithmeticIntegerNumeralText(strconv.FormatInt(int64(upper), 10)))
	if err != nil {
		return output, err
	}
	above, err := pin(m.MkArithmeticGE(length, boundary))
	if err != nil {
		return output, err
	}
	moreEquivalent, err := pin(t.state.mkEqualityAtom(moreChoice, above))
	if err != nil {
		return output, err
	}
	equivalences = append(equivalences, moreEquivalent)
	disjunction, err := pin(m.MkOrArray(choices))
	if err != nil {
		return output, err
	}
	conjuncts := append([]ast.ASTHandle{disjunction}, equivalences...)
	formula, err := pin(m.MkAndArray(conjuncts))
	if err != nil {
		return output, err
	}
	if tries > 1 {
		prior := t.testers.lengthIndicators[variable]
		if int64(len(prior)) < int64(tries-1) {
			return output, ast.ErrASTDomain
		}
		var premises []ast.ASTHandle
		for index := int32(0); index < tries-1; index++ {
			previous := prior[index]
			if _, active := t.internalVariables[previous]; !active {
				continue
			}
			condition, err := pin(t.state.mkEqualityAtom(previous, more))
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
	} else if err = t.retain(formula); err != nil {
		return output, err
	}
	return m.NewRef(formula)
}
