package smtcore

// Source: src/smt/theory_str.cpp process_concat_eq_type1,
// mk_nonempty_str_var, refresh_theory_var, and finite overlap tester setup.

import (
	"errors"
	"math/big"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (t *SMTStringTheory) addStringBranchPriority(term ast.ASTHandle, priority float64) error {
	if err := t.state.Internalize(term, false); err != nil {
		return err
	}
	variable, found := t.state.BoolVar(term)
	if !found {
		return ast.ErrASTDomain
	}
	return t.state.AddTheoryAwareBranchingInfo(variable, priority, lbool.True)
}

func (t *SMTStringTheory) addNonemptyStringConstraint(variable ast.ASTHandle) (result error) {
	m := t.state.manager
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	empty, err := t.stringLiteral("")
	if err != nil {
		return err
	}
	equality, err := m.MkEq(variable, empty)
	if err != nil {
		return err
	}
	if err := locals.add(equality); err != nil {
		return err
	}
	negative, err := m.MkNot(equality)
	if err != nil {
		return err
	}
	if err := t.assertAxiom(negative); err != nil {
		return err
	}
	length, err := m.MkSequenceLength(variable)
	if err != nil {
		return err
	}
	if err := locals.add(length); err != nil {
		return err
	}
	zero, err := m.ArithmeticIntegerNumeralText("0")
	if err != nil {
		return err
	}
	bound, err := m.MkArithmeticLE(length, zero)
	if err != nil {
		return err
	}
	if err := locals.add(bound); err != nil {
		return err
	}
	positive, err := m.MkNot(bound)
	if err != nil {
		return err
	}
	return t.assertAxiom(positive)
}

func (t *SMTStringTheory) freshNonemptyString() (ast.ASTHandle, error) {
	name := "$$_str" + strconv.FormatUint(uint64(t.nonemptyID), 10)
	t.nonemptyID++
	variable, err := t.createStringVariable(name, false)
	if err != nil {
		return variable, err
	}
	return variable, t.addNonemptyStringConstraint(variable)
}

func (t *SMTStringTheory) refreshStringVariable(variable ast.ASTHandle) error {
	if err := t.state.Internalize(variable, false); err != nil {
		return err
	}
	node := t.state.context.enodes[variable]
	if _, err := t.mkVar(node); err != nil {
		return err
	}
	t.basicTodo = append(t.basicTodo, node)
	return nil
}

func (t *SMTStringTheory) stringConcatWitnesses(left, right ast.ASTHandle) (variables [3]ast.ASTHandle, result error) {
	key := [2]ast.ASTHandle{left, right}
	for _, candidate := range [][2]ast.ASTHandle{key, {right, left}} {
		entry, found := t.arrangementVars[candidate]
		_, first := t.internalVariables[entry[0]]
		_, second := t.internalVariables[entry[1]]
		if !found || !first || !second {
			continue
		}
		for _, variable := range entry[:2] {
			if err := t.refreshStringVariable(variable); err != nil {
				return variables, err
			}
			if err := t.addNonemptyStringConstraint(variable); err != nil {
				return variables, err
			}
		}
		// Source reverse-key reuse preserves t1/t2 rather than swapping them.
		return entry, nil
	}
	var err error
	for i := 0; i < 2; i++ {
		variables[i], err = t.freshNonemptyString()
		if err != nil {
			return variables, err
		}
	}
	// The retained source allocates this integer flag even though type 1
	// does not subsequently use its value.
	variables[2], err = t.freshIntegerVariable("$$_xor")
	if err != nil {
		return variables, err
	}
	for _, variable := range variables[:2] {
		if err := t.initializeStringCut(variable); err != nil {
			return variables, err
		}
	}
	t.arrangementVars[key] = variables
	return variables, nil
}

func (t *SMTStringTheory) finiteStringOverlapTest() (out ast.ASTHandle, result error) {
	m := t.state.manager
	variable, err := t.freshStringVariable("finiteModelTest")
	if err != nil {
		return out, err
	}
	if err := t.retain(variable); err != nil {
		return out, err
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	empty, err := t.stringLiteral("")
	if err != nil {
		return out, err
	}
	yes, err := t.stringLiteral("yes")
	if err != nil {
		return out, err
	}
	first, err := m.MkEq(variable, empty)
	if err != nil {
		return out, err
	}
	if err := locals.add(first); err != nil {
		return out, err
	}
	out, err = m.MkEq(variable, yes)
	if err != nil {
		return out, err
	}
	if err := locals.add(out); err != nil {
		return out, err
	}
	choice, err := m.MkOr(first, out)
	if err != nil {
		return out, err
	}
	// Native finite_model_test_varlists is read only by finite_model_test,
	// whose sole equality-dispatch call is commented out. Its classification
	// and unused varlist storage have no retained semantic consumer.
	return out, t.assertAxiom(choice)
}

func (t *SMTStringTheory) stringOverlapAlternative(parameters SMTStrState) (ast.ASTHandle, error) {
	t.loopDetected = true
	if parameters.FiniteOverlapModels {
		return t.finiteStringOverlapTest()
	}
	if t.overlapAssumption == (ast.ASTRef{}) {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	return t.state.manager.RefGet(t.overlapAssumption)
}

func (t *SMTStringTheory) publishStringArrangement(premise, conclusion ast.ASTHandle, strong bool) (result error) {
	if strong {
		axiom, err := t.state.manager.MkEq(premise, conclusion)
		if err != nil {
			return err
		}
		return t.assertAxiom(axiom)
	}
	guards := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, guards.close()) }()
	if err := guards.add(premise); err != nil {
		return err
	}
	return guards.imply(conclusion)
}

func (t *SMTStringTheory) strictStringLengthGreater(left, right ast.ASTHandle) (out ast.ASTHandle, result error) {
	m := t.state.manager
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	l, err := pin(m.MkSequenceLength(left))
	if err != nil {
		return out, err
	}
	r, err := pin(m.MkSequenceLength(right))
	if err != nil {
		return out, err
	}
	minusOne, err := pin(m.ArithmeticIntegerNumeralText("-1"))
	if err != nil {
		return out, err
	}
	negative, err := pin(m.MkArithmeticMul(minusOne, r))
	if err != nil {
		return out, err
	}
	difference, err := pin(m.MkArithmeticAdd(l, negative))
	if err != nil {
		return out, err
	}
	zero, err := pin(m.ArithmeticIntegerNumeralText("0"))
	if err != nil {
		return out, err
	}
	bound, err := pin(m.MkArithmeticLE(difference, zero))
	if err != nil {
		return out, err
	}
	return m.MkNot(bound)
}

// Build the source unknown-length option "otherPrefix cuts tail". The
// second option uses the same concrete construction with the sides reversed.
func (t *SMTStringTheory) stringCutOption(prefix, tail, otherPrefix, otherTail, witness ast.ASTHandle) (out ast.ASTHandle, result error) {
	m := t.state.manager
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	first, err := pin(t.concatTerm(prefix, witness))
	if err != nil {
		return out, err
	}
	second, err := pin(t.concatTerm(witness, otherTail))
	if err != nil {
		return out, err
	}
	items := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, items.close()) }()
	if err := items.equate(otherPrefix, first); err != nil {
		return out, err
	}
	if err := items.equate(tail, second); err != nil {
		return out, err
	}
	l, err := pin(m.MkSequenceLength(prefix))
	if err != nil {
		return out, err
	}
	r, err := pin(m.MkSequenceLength(witness))
	if err != nil {
		return out, err
	}
	sum, err := pin(m.MkArithmeticAdd(l, r))
	if err != nil {
		return out, err
	}
	total, err := pin(m.MkSequenceLength(otherPrefix))
	if err != nil {
		return out, err
	}
	if err := items.equate(total, sum); err != nil {
		return out, err
	}
	for _, pair := range [][2]ast.ASTHandle{{otherPrefix, prefix}, {tail, otherTail}} {
		greater, err := t.strictStringLengthGreater(pair[0], pair[1])
		if err != nil {
			return out, err
		}
		if err := items.add(greater); err != nil {
			return out, err
		}
	}
	return m.MkAndArray(items.values)
}

func (t *SMTStringTheory) arrangeVariableConcats(left, right ast.ASTHandle) (result error) {
	m := t.state.manager
	x, y, lc, err := m.SequenceConcatParts(left)
	if err != nil || !lc {
		return err
	}
	a, b, rc, err := m.SequenceConcatParts(right)
	if err != nil || !rc {
		return err
	}
	parameters := SMTStrState{StrongArrangements: true, OverlapTheoryAwarePriority: -0.1}
	if t.parameterOwner != nil {
		parameters, err = t.stringParameterState()
		if err != nil {
			return err
		}
	}
	values := [4]*big.Int{}
	known := [4]bool{}
	for i, expression := range []ast.ASTHandle{x, y, a, b} {
		values[i], known[i], err = t.lengthValue(expression)
		if err != nil {
			return err
		}
	}
	split := -1
	if known[0] && known[2] {
		split = values[0].Cmp(values[2]) + 1
	} else if known[1] && known[3] {
		split = 1 - values[1].Cmp(values[3])
	}
	variables, err := t.stringConcatWitnesses(left, right)
	if err != nil {
		return err
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	parentEq, err := pin(m.MkEq(left, right))
	if err != nil {
		return err
	}
	if split == 1 {
		var lengths []ast.ASTHandle
		for _, pair := range [][2]ast.ASTHandle{{x, a}, {y, b}} {
			l, err := pin(m.MkSequenceLength(pair[0]))
			if err != nil {
				return err
			}
			r, err := pin(m.MkSequenceLength(pair[1]))
			if err != nil {
				return err
			}
			eq, err := pin(m.MkEq(l, r))
			if err != nil {
				return err
			}
			lengths = append(lengths, eq)
		}
		equalLength, err := pin(m.MkOrArray(lengths))
		if err != nil {
			return err
		}
		premise, err := pin(m.MkAnd(parentEq, equalLength))
		if err != nil {
			return err
		}
		first, err := pin(m.MkEq(x, a))
		if err != nil {
			return err
		}
		second, err := pin(m.MkEq(y, b))
		if err != nil {
			return err
		}
		conclusion, err := pin(m.MkAnd(first, second))
		if err != nil {
			return err
		}
		return t.publishStringArrangement(premise, conclusion, false)
	}
	if split == 0 || split == 2 {
		witness, cutter, cut, prefix, tail := variables[0], a, y, x, b
		if split == 2 {
			witness, cutter, cut, prefix, tail = variables[1], x, b, a, y
		}
		first, err := pin(t.concatTerm(prefix, witness))
		if err != nil {
			return err
		}
		second, err := pin(t.concatTerm(witness, tail))
		if err != nil {
			return err
		}
		guards := smtStringLengthPremises{theory: t}
		defer func() { result = errors.Join(result, guards.close()) }()
		conclusions := smtStringLengthPremises{theory: t}
		defer func() { result = errors.Join(result, conclusions.close()) }()
		if err := guards.add(parentEq); err != nil {
			return err
		}
		if err := conclusions.equate(cutter, first); err != nil {
			return err
		}
		if split == 0 {
			err = conclusions.equate(cut, second)
		} else {
			err = conclusions.equate(second, cut)
		}
		if err != nil {
			return err
		}
		p, q := 0, 2
		terms := [4]ast.ASTHandle{x, y, a, b}
		if !known[0] || !known[2] {
			p, q = 1, 3
		}
		if err := guards.length(terms[p], values[p]); err != nil {
			return err
		}
		if err := guards.length(terms[q], values[q]); err != nil {
			return err
		}
		difference := new(big.Int).Sub(values[q], values[p])
		if (split == 2) != (p == 1) {
			difference.Neg(difference)
		}
		if err := conclusions.length(witness, difference); err != nil {
			return err
		}
		premise, err := pin(m.MkAndArray(guards.values))
		if err != nil {
			return err
		}
		conclusion, err := pin(m.MkAndArray(conclusions.values))
		if err != nil {
			return err
		}
		if t.hasSelfCut(cutter, cut) {
			tester, err := t.stringOverlapAlternative(parameters)
			if err != nil {
				return err
			}
			if err := t.publishStringArrangement(premise, tester, false); err != nil {
				return err
			}
			if parameters.FiniteOverlapModels {
				return t.addStringBranchPriority(tester, parameters.OverlapTheoryAwarePriority)
			}
			return nil
		}
		if err := t.mergeStringCut(witness, int64(t.level), cutter); err != nil {
			return err
		}
		if err := t.mergeStringCut(witness, int64(t.level), cut); err != nil {
			return err
		}
		return t.publishStringArrangement(premise, conclusion, parameters.StrongArrangements)
	}
	var options []ast.ASTHandle
	overlapUsed := false
	for i, args := range [][4]ast.ASTHandle{{x, y, a, b}, {a, b, x, y}} {
		if !t.hasSelfCut(args[2], args[1]) {
			option, err := pin(t.stringCutOption(args[0], args[1], args[2], args[3], variables[i]))
			if err != nil {
				return err
			}
			options = append(options, option)
			if err := t.addStringBranchPriority(option, 0.1); err != nil {
				return err
			}
			for _, origin := range []ast.ASTHandle{args[2], args[1]} {
				if err := t.mergeStringCut(variables[i], int64(t.state.scopeLevel), origin); err != nil {
					return err
				}
			}
		} else {
			t.loopDetected = true
			if parameters.FiniteOverlapModels || !overlapUsed {
				tester, err := pin(t.stringOverlapAlternative(parameters))
				if err != nil {
					return err
				}
				options = append(options, tester)
				if parameters.FiniteOverlapModels {
					if err := t.addStringBranchPriority(tester, parameters.OverlapTheoryAwarePriority); err != nil {
						return err
					}
				}
				if !parameters.FiniteOverlapModels {
					overlapUsed = true
				}
			}
		}
	}
	firstPossible, err := t.canTwoNodesEqual(x, a)
	if err != nil {
		return err
	}
	secondPossible, err := t.canTwoNodesEqual(y, b)
	if err != nil {
		return err
	}
	if firstPossible && secondPossible {
		items := smtStringLengthPremises{theory: t}
		defer func() { result = errors.Join(result, items.close()) }()
		for _, pair := range [][2]ast.ASTHandle{{x, a}, {y, b}} {
			if err := items.equate(pair[0], pair[1]); err != nil {
				return err
			}
		}
		for _, pair := range [][2]ast.ASTHandle{{x, a}, {y, b}} {
			l, err := pin(m.MkSequenceLength(pair[0]))
			if err != nil {
				return err
			}
			r, err := pin(m.MkSequenceLength(pair[1]))
			if err != nil {
				return err
			}
			if err := items.equate(l, r); err != nil {
				return err
			}
		}
		option, err := pin(m.MkAndArray(items.values))
		if err != nil {
			return err
		}
		options = append(options, option)
		if err := t.addStringBranchPriority(option, 0.5); err != nil {
			return err
		}
	}
	if len(options) == 0 {
		return nil
	}
	conclusion, err := pin(m.MkOrArray(options))
	if err != nil {
		return err
	}
	if err := t.publishStringArrangement(parentEq, conclusion, parameters.StrongArrangements); err != nil {
		return err
	}
	literals := make([]SMTLiteral, len(options))
	for i, option := range options {
		literals[i], err = t.state.Literal(option)
		if err != nil {
			return err
		}
	}
	return t.state.MakeTheoryCaseSplit(literals)
}
