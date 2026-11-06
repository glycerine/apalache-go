package smtcore

// Source: src/smt/theory_str.cpp mk_unroll_test_var, get_unrolled_string,
// gen_unroll_assign and gen_unroll_conditional_options.

import (
	"errors"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

type smtStringUnrollHistory struct {
	unrolls []ast.ASTHandle
	testers []ast.ASTHandle
}

func (t *SMTStringTheory) freshStringUnrollTester() (ast.ASTHandle, error) {
	variable, err := t.freshStringVariable("unrollTest")
	if err != nil {
		return variable, err
	}
	t.unrollTestVariables[variable] = struct{}{}
	// Native repeats track_variable_scope, an idempotent insertion already
	// performed by mk_str_var. Unlike length/value testers, no extra root.
	t.variableScopes[t.level][variable] = struct{}{}
	return variable, nil
}

func (t *SMTStringTheory) generateStringUnrollAssignment(word ast.ASTHandle, core string, tester ast.ASTHandle, lower, upper int32) (output ast.ASTRef, result error) {
	if lower < 0 || upper < lower || uint64(len(core)) > 1<<32-1 {
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
	options := SMTStrState{}
	if t.parameterOwner != nil {
		var err error
		options, err = t.stringParameterState()
		if err != nil {
			return output, err
		}
	}
	var choices, conjuncts []ast.ASTHandle
	for i := lower; i < upper; i++ {
		label, err := pin(t.stringLiteral(strconv.FormatInt(int64(i), 10)))
		if err != nil {
			return output, err
		}
		choice, err := pin(t.state.mkEqualityAtom(tester, label))
		if err != nil {
			return output, err
		}
		if options.AggressiveUnrollTesting {
			if err = t.forceStringTesterPhase(tester, label, true); err != nil {
				return output, err
			}
		}
		choices = append(choices, choice)
		value, err := pin(t.stringLiteral(strings.Repeat(core, int(i))))
		if err != nil {
			return output, err
		}
		equal, err := pin(t.state.mkEqualityAtom(word, value))
		if err != nil {
			return output, err
		}
		equivalence, err := pin(t.state.mkEqualityAtom(choice, equal))
		if err != nil {
			return output, err
		}
		conjuncts = append(conjuncts, equivalence)
		length, err := pin(m.MkSequenceLength(word))
		if err != nil {
			return output, err
		}
		// Source multiplies by zstring's unsigned length and passes the
		// resulting target-width signed int to mk_int.
		n := int32(uint32(i) * uint32(len(core)))
		number, err := pin(m.ArithmeticIntegerNumeralText(strconv.FormatInt(int64(n), 10)))
		if err != nil {
			return output, err
		}
		exact, err := pin(t.state.mkEqualityAtom(length, number))
		if err != nil {
			return output, err
		}
		equivalence, err = pin(t.state.mkEqualityAtom(choice, exact))
		if err != nil {
			return output, err
		}
		conjuncts = append(conjuncts, equivalence)
	}
	more, err := pin(t.stringLiteral("more"))
	if err != nil {
		return output, err
	}
	moreChoice, err := pin(t.state.mkEqualityAtom(tester, more))
	if err != nil {
		return output, err
	}
	if options.AggressiveUnrollTesting {
		if err = t.forceStringTesterPhase(tester, more, false); err != nil {
			return output, err
		}
	}
	choices = append(choices, moreChoice)
	next := int32(uint32(upper) * uint32(len(core)))
	if next == -1<<31 {
		return output, ast.ErrASTDomain
	} // Native signed negation overflow.
	length, err := pin(m.MkSequenceLength(word))
	if err != nil {
		return output, err
	}
	negative, err := pin(m.ArithmeticIntegerNumeralText(strconv.FormatInt(-int64(next), 10)))
	if err != nil {
		return output, err
	}
	difference, err := pin(m.MkArithmeticAdd(length, negative))
	if err != nil {
		return output, err
	}
	zero, err := pin(m.ArithmeticIntegerNumeralText("0"))
	if err != nil {
		return output, err
	}
	above, err := pin(m.MkArithmeticGE(difference, zero))
	if err != nil {
		return output, err
	}
	equivalence, err := pin(t.state.mkEqualityAtom(moreChoice, above))
	if err != nil {
		return output, err
	}
	conjuncts = append(conjuncts, equivalence)
	disjunction, err := pin(m.MkOrArray(choices))
	if err != nil {
		return output, err
	}
	conjuncts = append(conjuncts, disjunction)
	formula, err := pin(m.MkAndArray(conjuncts))
	if err != nil {
		return output, err
	}
	if err = t.retain(formula); err != nil {
		return output, err
	}
	return m.NewRef(formula)
}

func (t *SMTStringTheory) generateStringConditionalUnrollOptions(word ast.ASTHandle, unrolls []ast.ASTHandle, core string) (output ast.ASTRef, result error) {
	// std::set keys use exact member identity, not private-class identity.
	key := append([]ast.ASTHandle(nil), unrolls...)
	stringSortHandles(key)
	unique := key[:0]
	for _, term := range key {
		if len(unique) == 0 || term != unique[len(unique)-1] {
			unique = append(unique, term)
		}
	}
	key = unique
	m := t.state.manager
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	var premises []ast.ASTHandle
	more, err := pin(t.stringLiteral("more"))
	if err != nil {
		return output, err
	}
	for _, unroll := range key {
		equal, err := pin(t.state.mkEqualityAtom(word, unroll))
		if err != nil {
			return output, err
		}
		premises = append(premises, equal)
	}
	var history *smtStringUnrollHistory
	for _, entry := range t.testers.unrollHistories[word] {
		if len(entry.unrolls) != len(key) {
			continue
		}
		equal := true
		for i := range key {
			if entry.unrolls[i] != key[i] {
				equal = false
				break
			}
		}
		if equal {
			history = entry
			break
		}
	}
	if history == nil {
		history = &smtStringUnrollHistory{unrolls: key}
		t.testers.unrollHistories[word] = append(t.testers.unrollHistories[word], history)
	}
	active := history.testers[:0]
	for _, tester := range history.testers {
		if _, ok := t.unrollTestVariables[tester]; ok {
			active = append(active, tester)
		}
	}
	history.testers = active
	if len(history.testers) == 0 {
		tester, err := t.freshStringUnrollTester()
		if err != nil {
			return output, err
		}
		history.testers = append(history.testers, tester)
	}
	produce := func(tester ast.ASTHandle, index int) (ast.ASTRef, error) {
		// opt_LCMUnrollStep is constructor-fixed at two, with no setter.
		if int64(index+1)*2 > 1<<31-1 {
			return ast.ASTRef{}, ast.ErrASTDomain
		}
		guard, err := pin(m.MkAndArray(premises))
		if err != nil {
			return ast.ASTRef{}, err
		}
		root, err := t.generateStringUnrollAssignment(word, core, tester, int32(index)*2, int32(index+1)*2)
		if err != nil {
			return ast.ASTRef{}, err
		}
		body, err := m.RefGet(root)
		if err == nil {
			err = locals.add(body)
		}
		err = errors.Join(err, m.DelRef(&root))
		if err != nil {
			return ast.ASTRef{}, err
		}
		negative, err := pin(m.MkNot(guard))
		if err != nil {
			return ast.ASTRef{}, err
		}
		formula, err := pin(m.MkOr(negative, body))
		if err != nil {
			return ast.ASTRef{}, err
		}
		if err = t.retain(formula); err != nil {
			return ast.ASTRef{}, err
		}
		return m.NewRef(formula)
	}
	for index, tester := range history.testers {
		if err = t.refreshStringVariable(tester); err != nil {
			return output, err
		}
		value, assigned, err := t.eqcStringValue(tester)
		if err != nil {
			return output, err
		}
		if !assigned {
			return produce(tester, index)
		} // Native returns before adding a later-more premise.
		text, _, err := m.SequenceStringValue(value)
		if err != nil {
			return output, err
		}
		if text == "more" {
			equal, err := pin(t.state.mkEqualityAtom(tester, more))
			if err != nil {
				return output, err
			}
			premises = append(premises, equal)
		}
	}
	tester, err := t.freshStringUnrollTester()
	if err != nil {
		return output, err
	}
	index := len(history.testers)
	history.testers = append(history.testers, tester)
	return produce(tester, index)
}
