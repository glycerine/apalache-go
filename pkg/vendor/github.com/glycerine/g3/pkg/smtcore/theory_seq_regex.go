package smtcore

// Source: src/smt/theory_seq.cpp propagate_in_re. The retained Go sequence
// rewriter supplies the equivalent automaton membership formula.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

const smtSequenceMaximumBoundedRegexLengths = 32

func (t *SMTSequenceTheory) sequenceLengthLowerBound(sequence ast.ASTHandle) (uint64, bool, error) {
	state, err := t.State()
	if err != nil {
		return 0, false, err
	}
	arithmeticFamily, err := state.manager.ArithmeticFamilyID()
	if err != nil {
		return 0, false, err
	}
	arithmetic, ok := state.Theory(arithmeticFamily).(*SMTLinearArithmeticTheory)
	if !ok {
		return 0, false, nil
	}
	intervals, err := arithmetic.linearVariableIntervals()
	if err != nil {
		return 0, false, err
	}
	length, err := state.manager.MkSequenceLength(sequence)
	if err != nil {
		return 0, false, err
	}
	interval, evaluated, err := arithmetic.polynomialInterval(length, intervals)
	if err != nil || !evaluated || interval.minusInfinity() {
		return 0, false, err
	}
	lowerValue, err := interval.lowerValue()
	if err != nil || lowerValue.Sign() < 0 {
		return 0, false, err
	}
	epsilonSign := 0
	if interval.lowerOpen {
		epsilonSign = 1
	}
	lower, integral := smtSequenceIntegerLowerBound(lowerValue, epsilonSign)
	return lower, integral, nil
}

func (t *SMTSequenceTheory) addRegexMembershipAxiom(expression ast.ASTHandle) (reducedExactly bool, result error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	sequence, err := state.manager.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	regex, err := state.manager.GetArg(expression, 1)
	if err != nil {
		return false, err
	}
	return t.addRegexMembershipAxiomForSequence(expression, sequence, regex)
}

func (t *SMTSequenceTheory) addRegexMembershipAxiomForSequence(expression, sequence, regex ast.ASTHandle) (reducedExactly bool, result error) {
	return t.addGuardedRegexMembershipAxiom(expression, sequence, regex, ast.ASTHandle{})
}

func (t *SMTSequenceTheory) addGuardedRegexMembershipAxiom(expression, sequence, regex, guard ast.ASTHandle) (reducedExactly bool, result error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	rw, err := rewriter.NewSequenceRewriter(state.manager)
	if err != nil {
		return false, err
	}
	out, err := state.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, state.manager.DelRef(&out)) }()
	status, err := rw.RewriteStringInRegex(sequence, regex, out)
	if err != nil {
		return false, err
	}
	if status == rewriter.RewriteFailed {
		return false, nil
	}
	reduced, err := state.manager.RefGet(out)
	if err != nil {
		return false, err
	}
	if reduced == expression {
		return false, nil
	}
	equivalence, err := state.manager.MkEq(expression, reduced)
	if err != nil {
		return false, err
	}
	if !guard.IsNull() {
		equivalence, err = state.manager.MkImplies(guard, equivalence)
		if err != nil {
			return false, err
		}
	}
	return true, t.axioms.Push(equivalence)
}

func (t *SMTSequenceTheory) fixedLengthRegexWord(sequence ast.ASTHandle, count uint64) (word, guard ast.ASTHandle, internal []ast.ASTHandle, result error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, nil, err
	}
	sort, err := state.manager.GetSort(sequence)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, nil, err
	}
	elementSort, isSequence, err := state.manager.SequenceSortElement(sort)
	if err != nil || !isSequence {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, ast.ASTHandle{}, nil, err
	}
	parts := make([]ast.ASTHandle, 0, int(count))
	internal = make([]ast.ASTHandle, 0, int(count)*2+1)
	for index := uint64(0); index < count; index++ {
		position, makeErr := t.integer(index)
		if makeErr != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, nil, makeErr
		}
		element, makeErr := t.relationSkolem("seq.nth", []ast.ASTHandle{sequence, position}, elementSort)
		if makeErr != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, nil, makeErr
		}
		unit, makeErr := state.manager.MkSequenceUnit(element)
		if makeErr != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, nil, makeErr
		}
		parts = append(parts, unit)
		internal = append(internal, element, unit)
	}
	word, err = t.makeWord(parts, sort)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, nil, err
	}
	length, err := state.manager.MkSequenceLength(sequence)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, nil, err
	}
	lengthValue, err := t.integer(count)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, nil, err
	}
	guard, err = state.manager.MkEq(length, lengthValue)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, nil, err
	}
	internal = append(internal, word)
	return word, guard, internal, nil
}

func (t *SMTSequenceTheory) boundedSequenceLengthRange(sequence ast.ASTHandle) (lower, upper uint64, bounded bool, result error) {
	state, err := t.State()
	if err != nil {
		return 0, 0, false, err
	}
	arithmeticFamily, err := state.manager.ArithmeticFamilyID()
	if err != nil {
		return 0, 0, false, err
	}
	arithmetic, ok := state.Theory(arithmeticFamily).(*SMTLinearArithmeticTheory)
	if !ok {
		return 0, 0, false, nil
	}
	intervals, err := arithmetic.linearVariableIntervals()
	if err != nil {
		return 0, 0, false, err
	}
	length, err := state.manager.MkSequenceLength(sequence)
	if err != nil {
		return 0, 0, false, err
	}
	interval, evaluated, err := arithmetic.polynomialInterval(length, intervals)
	if err != nil || !evaluated || interval.plusInfinity() {
		return 0, 0, false, err
	}
	upperValue, err := interval.upperValue()
	if err != nil {
		return 0, 0, false, err
	}
	upper, integral := smtSequenceIntegerUpperBound(upperValue, interval.upperOpen)
	if !integral || upper > smtSequenceMaximumFixedDecomposition {
		return 0, 0, false, nil
	}
	lower = 0
	if !interval.minusInfinity() {
		lowerValue, valueErr := interval.lowerValue()
		if valueErr != nil {
			return 0, 0, false, valueErr
		}
		if lowerValue.Sign() >= 0 {
			epsilonSign := 0
			if interval.lowerOpen {
				epsilonSign = 1
			}
			lower, integral = smtSequenceIntegerLowerBound(lowerValue, epsilonSign)
			if !integral {
				return 0, 0, false, nil
			}
		}
	}
	if lower > upper {
		return 0, 0, false, nil
	}
	if upper-lower >= smtSequenceMaximumBoundedRegexLengths {
		return 0, 0, false, nil
	}
	return lower, upper, true, nil
}

func (t *SMTSequenceTheory) addBoundedRegexMembershipAxioms(expression, sequence, regex ast.ASTHandle, lower, upper uint64) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	allReduced := true
	for count := lower; ; count++ {
		word, guard, internal, wordErr := t.fixedLengthRegexWord(sequence, count)
		if wordErr != nil {
			return false, wordErr
		}
		definition, makeErr := state.manager.MkEq(sequence, word)
		if makeErr != nil {
			return false, makeErr
		}
		guardedDefinition, makeErr := state.manager.MkImplies(guard, definition)
		if makeErr != nil {
			return false, makeErr
		}
		if _, found := t.axioms.set[guardedDefinition]; !found {
			if makeErr = t.axioms.Push(guardedDefinition); makeErr != nil {
				return false, makeErr
			}
		}
		reduced, reduceErr := t.addGuardedRegexMembershipAxiom(expression, word, regex, guard)
		if reduceErr != nil {
			return false, reduceErr
		}
		if !reduced {
			allReduced = false
		}
		t.trackInternalTerms(internal...)
		if count == upper {
			break
		}
	}
	return allReduced, nil
}

func (t *SMTSequenceTheory) branchConcreteRegexWitness(expression, sequence, regex ast.ASTHandle) (branched, reduced bool, result error) {
	state, err := t.State()
	if err != nil {
		return false, false, err
	}
	literal, err := state.Literal(expression)
	if err != nil {
		return false, false, err
	}
	assignment, err := state.Assignment(literal)
	if err != nil || assignment == lbool.Undef {
		return false, false, err
	}
	candidateRegex := regex
	activation := expression
	if assignment == lbool.False {
		candidateRegex, err = state.manager.MkRegexComplement(regex)
		if err != nil {
			return false, false, err
		}
		activation, err = state.manager.MkNot(expression)
		if err != nil {
			return false, false, err
		}
	}
	rw, err := rewriter.NewSequenceRewriter(state.manager)
	if err != nil {
		return false, false, err
	}
	lower, hasLower, err := t.sequenceLengthLowerBound(sequence)
	if err != nil {
		return false, false, err
	}
	if hasLower && lower > smtSequenceMaximumFixedDecomposition {
		return false, false, nil
	}
	limit := len(state.boolExpr) + 1
	if hasLower {
		limit += int(lower)
	}
	witnesses, exhaustive, err := rw.RegexByteStringCandidates(candidateRegex, limit)
	if err != nil {
		return false, false, err
	}
	sequenceSort, err := state.manager.GetSort(sequence)
	if err != nil {
		return false, false, err
	}
	equalities := make([]ast.ASTHandle, 0, len(witnesses))
	for _, witnessText := range witnesses {
		witness, witnessErr := state.manager.MkString(witnessText)
		if witnessErr != nil {
			return false, false, witnessErr
		}
		witnessSort, witnessErr := state.manager.GetSort(witness)
		if witnessErr != nil || witnessSort != sequenceSort {
			return false, false, witnessErr
		}
		equality, witnessErr := state.manager.MkEq(sequence, witness)
		if witnessErr != nil {
			return false, false, witnessErr
		}
		equalities = append(equalities, equality)
		if hasLower && uint64(len(witnessText)) < lower {
			continue
		}
		equalityAssignment, witnessErr := t.assumeSequenceEquality(sequence, witness)
		if witnessErr != nil {
			return false, false, witnessErr
		}
		if equalityAssignment == lbool.Undef {
			return true, false, nil
		}
		if equalityAssignment == lbool.True {
			reduced, witnessErr = t.addGuardedRegexMembershipAxiom(expression, witness, regex, equality)
			return false, reduced, witnessErr
		}
	}
	if !exhaustive {
		return false, false, nil
	}
	choices, err := state.manager.MkFalse()
	if err != nil {
		return false, false, err
	}
	if len(equalities) != 0 {
		choices, err = state.manager.MkOrArray(equalities)
		if err != nil {
			return false, false, err
		}
	}
	constraint, err := state.manager.MkImplies(activation, choices)
	if err != nil {
		return false, false, err
	}
	if _, found := t.axioms.set[constraint]; found {
		return false, false, nil
	}
	return true, true, t.axioms.Push(constraint)
}

func (t *SMTSequenceTheory) assumeRegexLength(guard ast.ASTHandle) (lbool.LBool, error) {
	state, err := t.State()
	if err != nil {
		return lbool.Undef, err
	}
	if err = state.Internalize(guard, false); err != nil {
		return lbool.Undef, err
	}
	literal, err := state.Literal(guard)
	if err != nil {
		return lbool.Undef, err
	}
	assignment, err := state.Assignment(literal)
	if err != nil || assignment != lbool.Undef {
		return assignment, err
	}
	if err = state.markRelevant(guard); err != nil {
		return lbool.Undef, err
	}
	data, err := state.BooleanData(literal.Var())
	if err != nil {
		return lbool.Undef, err
	}
	data.SetTrueFirst()
	return lbool.Undef, nil
}

func (t *SMTSequenceTheory) branchSymbolicRegexLength(expression, sequence, regex ast.ASTHandle) (branched, reduced bool, result error) {
	state, err := t.State()
	if err != nil {
		return false, false, err
	}
	literal, err := state.Literal(expression)
	if err != nil {
		return false, false, err
	}
	assignment, err := state.Assignment(literal)
	if err != nil || assignment == lbool.Undef {
		return false, false, err
	}
	candidateRegex := regex
	activation := expression
	if assignment == lbool.False {
		candidateRegex, err = state.manager.MkRegexComplement(regex)
		if err != nil {
			return false, false, err
		}
		activation, err = state.manager.MkNot(expression)
		if err != nil {
			return false, false, err
		}
	}
	lower, hasLower, err := t.sequenceLengthLowerBound(sequence)
	if err != nil {
		return false, false, err
	}
	if hasLower && lower > smtSequenceMaximumFixedDecomposition {
		return false, false, nil
	}
	limit := len(state.boolExpr) + 1
	if hasLower {
		limit += int(lower)
	}
	rw, err := rewriter.NewSequenceRewriter(state.manager)
	if err != nil {
		return false, false, err
	}
	lengths, exhaustive, err := rw.RegexAcceptedLengths(candidateRegex, limit)
	if err != nil {
		return false, false, err
	}
	guards := make([]ast.ASTHandle, 0, len(lengths))
	for _, count := range lengths {
		if count > smtSequenceMaximumFixedDecomposition {
			return false, false, nil
		}
		word, guard, internal, wordErr := t.fixedLengthRegexWord(sequence, count)
		if wordErr != nil {
			return false, false, wordErr
		}
		guards = append(guards, guard)
		if hasLower && count < lower {
			continue
		}
		assignment, wordErr := t.assumeRegexLength(guard)
		if wordErr != nil {
			return false, false, wordErr
		}
		if assignment == lbool.False {
			continue
		}
		definition, wordErr := state.manager.MkEq(sequence, word)
		if wordErr != nil {
			return false, false, wordErr
		}
		guardedDefinition, wordErr := state.manager.MkImplies(guard, definition)
		if wordErr != nil {
			return false, false, wordErr
		}
		if _, found := t.axioms.set[guardedDefinition]; !found {
			if wordErr = t.axioms.Push(guardedDefinition); wordErr != nil {
				return false, false, wordErr
			}
		}
		t.trackInternalTerms(internal...)
		if assignment == lbool.Undef {
			return true, false, nil
		}
		reduced, wordErr = t.addGuardedRegexMembershipAxiom(expression, word, regex, guard)
		return false, reduced, wordErr
	}
	if !exhaustive {
		return false, false, nil
	}
	choices, err := state.manager.MkFalse()
	if err != nil {
		return false, false, err
	}
	if len(guards) != 0 {
		choices, err = state.manager.MkOrArray(guards)
		if err != nil {
			return false, false, err
		}
	}
	constraint, err := state.manager.MkImplies(activation, choices)
	if err != nil {
		return false, false, err
	}
	if _, found := t.axioms.set[constraint]; found {
		return false, false, nil
	}
	return true, true, t.axioms.Push(constraint)
}

func (t *SMTSequenceTheory) reduceExpandedRegexMemberships() (progress, unresolved bool, result error) {
	state, err := t.State()
	if err != nil {
		return false, false, err
	}
	seen := make(map[ast.ASTHandle]struct{})
	for expression := range state.context.enodes {
		if _, found := seen[expression]; found {
			continue
		}
		seen[expression] = struct{}{}
		operation, own, operationErr := t.operation(expression)
		if operationErr != nil {
			return false, false, operationErr
		}
		if !own || operation != 12 {
			continue
		}
		sequence, argumentErr := state.manager.GetArg(expression, 0)
		if argumentErr != nil {
			return false, false, argumentErr
		}
		regex, argumentErr := state.manager.GetArg(expression, 1)
		if argumentErr != nil {
			return false, false, argumentErr
		}
		parts, _, expandErr := t.expandWord([]ast.ASTHandle{sequence}, nil)
		if expandErr != nil {
			return false, false, expandErr
		}
		sort, sortErr := state.manager.GetSort(sequence)
		if sortErr != nil {
			return false, false, sortErr
		}
		expanded, makeErr := t.makeWord(parts, sort)
		if makeErr != nil {
			return false, false, makeErr
		}
		before := len(t.axioms.set)
		reduced, reduceErr := t.addRegexMembershipAxiomForSequence(expression, expanded, regex)
		if reduceErr != nil {
			return false, false, reduceErr
		}
		if !reduced {
			count, exact, lengthErr := t.exactSequenceLength(sequence)
			if lengthErr != nil {
				return false, false, lengthErr
			}
			if exact && count <= smtSequenceMaximumFixedDecomposition {
				fixedWord, guard, internal, wordErr := t.fixedLengthRegexWord(sequence, count)
				if wordErr != nil {
					return false, false, wordErr
				}
				reduced, reduceErr = t.addGuardedRegexMembershipAxiom(expression, fixedWord, regex, guard)
				if reduceErr != nil {
					return false, false, reduceErr
				}
				if reduced {
					t.trackInternalTerms(internal...)
				}
			} else if !exact {
				lower, upper, bounded, boundsErr := t.boundedSequenceLengthRange(sequence)
				if boundsErr != nil {
					return false, false, boundsErr
				}
				if bounded {
					reduced, reduceErr = t.addBoundedRegexMembershipAxioms(expression, sequence, regex, lower, upper)
					if reduceErr != nil {
						return false, false, reduceErr
					}
				}
			}
		}
		if !reduced {
			branched, witnessReduced, witnessErr := t.branchConcreteRegexWitness(expression, sequence, regex)
			if witnessErr != nil {
				return false, false, witnessErr
			}
			if branched {
				progress = true
				continue
			}
			reduced = witnessReduced
		}
		if !reduced {
			branched, lengthReduced, lengthErr := t.branchSymbolicRegexLength(expression, sequence, regex)
			if lengthErr != nil {
				return false, false, lengthErr
			}
			if branched {
				progress = true
				continue
			}
			reduced = lengthReduced
		}
		if !reduced {
			unresolved = true
			continue
		}
		t.trackInternalTerms(expression, expanded)
		t.trackInternalTerms(parts...)
		if treeErr := t.trackRegexTree(regex); treeErr != nil {
			return false, false, treeErr
		}
		if len(t.axioms.set) != before {
			progress = true
		}
	}
	return progress, unresolved, nil
}

func (t *SMTSequenceTheory) trackRegexTree(expression ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	sort, err := state.manager.GetSort(expression)
	if err != nil {
		return err
	}
	_, regex, err := t.sequenceSort(sort)
	if err != nil {
		return err
	}
	if !regex {
		return nil
	}
	t.trackInternalTerms(expression)
	count, err := state.manager.GetNumArgs(expression)
	if err != nil {
		return err
	}
	for index := uint32(0); index < count; index++ {
		argument, argumentErr := state.manager.GetArg(expression, index)
		if argumentErr != nil {
			return argumentErr
		}
		if argumentErr = t.trackRegexTree(argument); argumentErr != nil {
			return argumentErr
		}
	}
	return nil
}
