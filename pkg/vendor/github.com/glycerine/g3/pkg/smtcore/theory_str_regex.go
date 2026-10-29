package smtcore

// Source: src/smt/theory_str.cpp str2RegexStr, get_std_regex_str,
// instantiate_axiom_RegexIn and check_regex_in.

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) regexAutomataEnabled() (bool, error) {
	if t.parameterOwner == nil {
		return true, nil
	}
	parameters, err := t.stringParameterState()
	return parameters.RegexAutomata, err
}

// These are registry keys, not Go regular expressions. Preserve the native
// twelve escapes and its one-byte C-string construction's omission of NUL.
func stringRegexLiteralKey(text string) string {
	var key strings.Builder
	for i := 0; i < len(text); i++ {
		if strings.ContainsRune("\\^$.|?*+()[{", rune(text[i])) {
			key.WriteByte('\\')
		}
		if text[i] != 0 {
			key.WriteByte(text[i])
		}
	}
	return key.String()
}

func (t *SMTStringTheory) regexTextKey(regex ast.ASTHandle) (string, error) {
	m := t.state.manager
	op, own, err := t.operation(regex)
	if err != nil {
		return "", err
	}
	if !own {
		return "", ast.ErrASTDomain
	}
	arg := func(i uint32) (ast.ASTHandle, error) { return m.GetArg(regex, i) }
	switch op {
	case 11:
		word, err := arg(0)
		if err != nil {
			return "", err
		}
		text, _, err := m.SequenceStringValue(word)
		return stringRegexLiteralKey(text), err
	case 14, 17, 18:
		left, err := arg(0)
		if err != nil {
			return "", err
		}
		first, err := t.regexTextKey(left)
		if err != nil {
			return "", err
		}
		if op == 14 {
			return "(" + first + ")*", nil
		}
		right, err := arg(1)
		if err != nil {
			return "", err
		}
		second, err := t.regexTextKey(right)
		if err != nil {
			return "", err
		}
		if op == 17 {
			return "(" + first + ")(" + second + ")", nil
		}
		return "(" + first + ")|(" + second + ")", nil
	case 16:
		left, err := arg(0)
		if err != nil {
			return "", err
		}
		right, err := arg(1)
		if err != nil {
			return "", err
		}
		lo, _, err := m.SequenceStringValue(left)
		if err != nil {
			return "", err
		}
		hi, _, err := m.SequenceStringValue(right)
		return "[" + lo + "-" + hi + "]", err
	case 23:
		return "(.*)", nil
	case 24:
		return "str.allchar", nil
	default:
		return "", fmt.Errorf("string theory regex operation %d is not supported", op)
	}
}

func (t *SMTStringTheory) instantiateRegexMembership(node *SMTEnode) error {
	m := t.state.manager
	expression := node.Owner()
	if _, found := t.axiomatized[expression]; found {
		return nil
	}
	t.axiomatized[expression] = struct{}{}
	word, err := m.GetArg(expression, 0)
	if err != nil {
		return err
	}
	regex, err := m.GetArg(expression, 1)
	if err != nil {
		return err
	}
	key, err := t.regexTextKey(regex)
	if err != nil {
		return err
	}
	// Registry handles survive scope pops. Keep their Go raw handles owned
	// even in automata mode, where this call emits no retaining axiom.
	if err := t.retain(expression); err != nil {
		return err
	}
	if t.regexMemberships[word] == nil {
		t.regexMemberships[word] = make(map[string]ast.ASTHandle)
	}
	t.regexMemberships[word][key] = expression
	automata, err := t.regexAutomataEnabled()
	if err != nil {
		return err
	}
	if automata {
		t.regexTerms[expression] = struct{}{}
		t.regexTermsByString[word] = append(t.regexTermsByString[word], expression)
		return nil
	}
	return t.instantiateLegacyRegexAxiom(expression, word, regex)
}

func (t *SMTStringTheory) regexMembership(word, regex ast.ASTHandle) (ast.ASTHandle, error) {
	expression, err := t.state.manager.MkSequenceInRegex(word, regex)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err := t.state.Internalize(expression, false); err != nil {
		return ast.ASTHandle{}, err
	}
	return expression, t.setUpAxioms(expression)
}

func (t *SMTStringTheory) checkRegexNewEquality(left, right ast.ASTHandle) error {
	m := t.state.manager
	var members []ast.ASTHandle
	var constant ast.ASTHandle
	for _, side := range []ast.ASTHandle{left, right} {
		var last ast.ASTHandle
		for _, member := range t.stringClass(side) {
			members = append(members, member)
			if _, literal, err := m.SequenceStringValue(member); err != nil {
				return err
			} else if literal {
				last = member
			}
		}
		if constant.IsNull() {
			constant = last
		}
	}
	if constant.IsNull() {
		return nil
	}
	text, _, err := m.SequenceStringValue(constant)
	if err != nil {
		return err
	}
	for _, member := range members {
		var keys []string
		for key := range t.regexMemberships[member] {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			expression := t.regexMemberships[member][key]
			regex, err := m.GetArg(expression, 1)
			if err != nil {
				return err
			}
			nfa := t.regexNFACache[regex]
			if nfa == nil {
				nfa, err = newSMTStringNFA(m, regex)
				if err != nil {
					return err
				}
				t.regexNFACache[regex] = nfa
			}
			if !nfa.valid {
				return fmt.Errorf("string theory regex NFA is invalid")
			}
			if err := t.assertRegexMatch(member, constant, expression, nfa.matches(text)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *SMTStringTheory) assertRegexMatch(word, constant, expression ast.ASTHandle, matches bool) (result error) {
	m := t.state.manager
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	// Native check_regex_in always creates this equality, including self.
	equality, err := m.MkEq(word, constant)
	if err != nil {
		return err
	}
	if err := premises.add(equality); err != nil {
		return err
	}
	if !matches {
		expression, err = m.MkNot(expression)
		if err != nil {
			return err
		}
	}
	return premises.imply(expression)
}
