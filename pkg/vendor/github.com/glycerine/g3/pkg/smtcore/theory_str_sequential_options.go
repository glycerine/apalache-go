package smtcore

// Source: src/smt/theory_str.cpp gen_len_val_options_for_free_var's
// !m_UseBinarySearch branch and term_appears_as_subterm.

import (
	"github.com/glycerine/g3/pkg/ast"
	"sort"
)

func (t *SMTStringTheory) stringTermAppearsAsSubterm(needle, haystack ast.ASTHandle) (bool, error) {
	same, err := t.sameStringClass(needle, haystack)
	if err != nil || same {
		return same, err
	}
	m := t.state.manager
	kind, err := m.GetKind(haystack)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	count, err := m.GetNumArgs(haystack)
	if err != nil {
		return false, err
	}
	for index := uint32(0); index < count; index++ {
		child, err := m.GetArg(haystack, index)
		if err != nil {
			return false, err
		}
		found, err := t.stringTermAppearsAsSubterm(needle, child)
		if err != nil || found {
			return found, err
		}
	}
	return false, nil
}

func (t *SMTStringTheory) generateStringSequentialOptions(variable, callback ast.ASTHandle, callbackValue string) (ast.ASTRef, error) {
	_, counted := t.testers.lengthCounts[variable]
	previous := t.testers.lengthIndicators[variable]
	empty := !counted
	if !empty {
		empty = true
		for _, indicator := range previous {
			if _, active := t.internalVariables[indicator]; active {
				empty = false
				break
			}
		}
	}
	create := func(tries uint32) (ast.ASTHandle, error) {
		indicator, err := t.freshStringLengthTester(variable, int32(tries))
		if err != nil {
			return indicator, err
		}
		t.testers.lengthIndicators[variable] = append(t.testers.lengthIndicators[variable], indicator)
		t.testers.lengthSubjects[indicator] = variable
		return indicator, nil
	}
	if empty {
		t.testers.lengthCounts[variable] = 1
		// Native clears old length history only after creating the new tester.
		indicator, err := t.freshStringLengthTester(variable, 1)
		if err != nil {
			return ast.ASTRef{}, err
		}
		t.testers.lengthIndicators[variable] = []ast.ASTHandle{indicator}
		t.testers.lengthSubjects[indicator] = variable
		return t.generateStringLengthOptions(variable, indicator, 1)
	}
	if int64(len(previous)) > 1<<31-1 {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	var effective ast.ASTHandle
	effectiveValue := ""
	i := 0
	m := t.state.manager
	for ; i < len(previous); i++ {
		indicator := previous[i]
		if _, active := t.internalVariables[indicator]; !active {
			continue
		}
		value, assigned, err := t.eqcStringValue(indicator)
		if err != nil {
			return ast.ASTRef{}, err
		}
		if assigned {
			text, _, err := m.SequenceStringValue(value)
			if err != nil {
				return ast.ASTRef{}, err
			}
			if text != "more" {
				effective, effectiveValue = indicator, text
				break
			}
		} else if indicator != callback {
			if i > 0 {
				effective = previous[i-1]
				value, assigned, err := t.eqcStringValue(effective)
				if err != nil {
					return ast.ASTRef{}, err
				}
				if effective == callback {
					effectiveValue = callbackValue
				} else if assigned {
					effectiveValue, _, err = m.SequenceStringValue(value)
					if err != nil {
						return ast.ASTRef{}, err
					}
				} else {
					return ast.ASTRef{}, ast.ErrASTDomain
				} // Native NOT_IMPLEMENTED_YET.
			}
			break
		} else if callbackValue != "more" {
			effective, effectiveValue = indicator, callbackValue
			break
		}
	}
	if effectiveValue == "more" || effectiveValue == "" {
		var indicator ast.ASTHandle
		var tries uint32
		var err error
		if i == len(previous) {
			t.testers.lengthCounts[variable]++
			tries = t.testers.lengthCounts[variable]
			indicator, err = create(tries)
			if err != nil {
				return ast.ASTRef{}, err
			}
		} else {
			indicator = previous[i]
			if err = t.refreshStringVariable(indicator); err != nil {
				return ast.ASTRef{}, err
			}
			tries = uint32(i) + 1
		}
		return t.generateStringLengthOptions(variable, indicator, int32(tries))
	}
	automata, err := t.regexAutomataEnabled()
	if err != nil {
		return ast.ASTRef{}, err
	}
	if automata {
		var words []ast.ASTHandle
		for word := range t.regexMemberships {
			words = append(words, word)
		}
		stringSortHandles(words)
		for _, word := range words {
			var keys []string
			for key := range t.regexMemberships[word] {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				term, err := m.GetArg(t.regexMemberships[word][key], 0)
				if err != nil {
					return ast.ASTRef{}, err
				}
				found, err := t.stringTermAppearsAsSubterm(variable, term)
				if err != nil {
					return ast.ASTRef{}, err
				}
				if found {
					return ast.ASTRef{}, nil
				}
			}
		}
	}
	return t.generateStringFreeValueOptions(variable, effective, effectiveValue, ast.ASTHandle{}, "")
}
