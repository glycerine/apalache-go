package smtcore

// Source: src/smt/theory_seq.cpp and src/smt/theory_seq.h retained
// internalization, equality-state, scope, conflict, and model hooks.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// SMTSequenceTheory is the incremental sequence-theory shell.  It completely
// handles the equality fragment over sequence variables and literal/empty
// values.  Operations whose word-equation, arithmetic, or automata rules have
// not yet been installed set unsupported; FinalCheck then reports give-up.
type SMTSequenceTheory struct {
	*SMTTheoryBase
	dependencies   *smtSequenceDependencyManager
	solutions      *smtSequenceSolutionMap
	exclusions     *smtSequenceExclusionTable
	equations      *smtSequenceEquationQueue
	disequations   *smtSequenceDisequationQueue
	nonContains    *smtSequenceNonContainsQueue
	relations      *smtSequenceRelationQueue
	axioms         *smtSequenceAxiomQueue
	rewrite        *smtSequenceRewriterOwner
	internalTerms  map[ast.ASTHandle]struct{}
	internalLog    []ast.ASTHandle
	internalLim    []int
	nextEquation   uint32
	branchStart    map[uint32]int
	branchLog      []uint32
	branchLim      []int
	lengths        map[ast.ASTHandle]struct{}
	lengthLog      []ast.ASTRef
	lengthLim      []int
	unsupported    uint32
	limits         []uint32
	statistics     smtSequenceStatistics
	branchProgress bool
}

func NewSMTSequenceTheory(family ast.FamilyID) (*SMTSequenceTheory, error) {
	base, err := NewSMTTheoryBase(family)
	if err != nil {
		return nil, err
	}
	return &SMTSequenceTheory{SMTTheoryBase: base, internalTerms: make(map[ast.ASTHandle]struct{}), branchStart: make(map[uint32]int), lengths: make(map[ast.ASTHandle]struct{})}, nil
}

func (t *SMTSequenceTheory) Initialize(state *SMTInternalizerState) error {
	if t == nil || t.dependencies != nil || t.solutions != nil || t.exclusions != nil || t.equations != nil || t.disequations != nil || t.nonContains != nil || t.relations != nil || t.axioms != nil {
		return ast.ErrASTHandle
	}
	if err := t.SMTTheoryBase.Initialize(state); err != nil {
		return err
	}
	t.dependencies = &smtSequenceDependencyManager{}
	var err error
	t.solutions, err = newSMTSequenceSolutionMap(state.manager, t.dependencies)
	if err != nil {
		return err
	}
	t.exclusions, err = newSMTSequenceExclusionTable(state.manager)
	if err != nil {
		_ = t.solutions.Close()
		t.solutions = nil
		return err
	}
	t.equations, err = newSMTSequenceEquationQueue(state.manager)
	if err != nil {
		_ = t.exclusions.Close()
		_ = t.solutions.Close()
		t.exclusions = nil
		t.solutions = nil
		return err
	}
	t.disequations, err = newSMTSequenceDisequationQueue(state.manager)
	if err != nil {
		_ = t.equations.Close()
		_ = t.exclusions.Close()
		_ = t.solutions.Close()
		t.equations = nil
		t.exclusions = nil
		t.solutions = nil
		return err
	}
	t.nonContains, err = newSMTSequenceNonContainsQueue(state.manager)
	if err != nil {
		_ = t.disequations.Close()
		_ = t.equations.Close()
		_ = t.exclusions.Close()
		_ = t.solutions.Close()
		t.disequations = nil
		t.equations = nil
		t.exclusions = nil
		t.solutions = nil
		return err
	}
	t.relations, err = newSMTSequenceRelationQueue(state.manager)
	if err != nil {
		_ = t.nonContains.Close()
		_ = t.disequations.Close()
		_ = t.equations.Close()
		_ = t.exclusions.Close()
		_ = t.solutions.Close()
		t.nonContains = nil
		t.disequations = nil
		t.equations = nil
		t.exclusions = nil
		t.solutions = nil
		return err
	}
	t.axioms, err = newSMTSequenceAxiomQueue(state.manager, &t.statistics)
	if err != nil {
		_ = t.relations.Close()
		_ = t.nonContains.Close()
		_ = t.disequations.Close()
		_ = t.equations.Close()
		_ = t.exclusions.Close()
		_ = t.solutions.Close()
		t.relations = nil
		t.nonContains = nil
		t.disequations = nil
		t.equations = nil
		t.exclusions = nil
		t.solutions = nil
		return err
	}
	return nil
}

func (t *SMTSequenceTheory) operation(expression ast.ASTHandle) (int32, bool, error) {
	state, err := t.State()
	if err != nil {
		return 0, false, err
	}
	family, err := state.manager.GetFamilyID(expression)
	if err != nil || family != t.FamilyID() {
		return 0, false, err
	}
	kind, err := state.manager.GetDeclKind(expression)
	return kind, err == nil, err
}

func (t *SMTSequenceTheory) sequenceSort(sort ast.ASTHandle) (sequence, regex bool, err error) {
	state, err := t.State()
	if err != nil {
		return false, false, err
	}
	_, sequence, err = state.manager.SequenceSortElement(sort)
	if err != nil || sequence {
		return sequence, false, err
	}
	_, regex, err = state.manager.RegexSequenceSort(sort)
	return false, regex, err
}

func (t *SMTSequenceTheory) supportedValue(expression ast.ASTHandle) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	if _, matched, err := state.manager.SequenceStringValue(expression); err != nil || matched {
		return matched, err
	}
	return state.manager.IsSequenceEmpty(expression)
}

func (t *SMTSequenceTheory) generatedSequenceTerm(expression ast.ASTHandle, visiting map[ast.ASTHandle]struct{}) (bool, error) {
	if _, found := visiting[expression]; found {
		return false, nil
	}
	visiting[expression] = struct{}{}
	defer delete(visiting, expression)
	op, own, err := t.operation(expression)
	if err != nil || !own {
		return false, err
	}
	if op == 42 {
		return true, nil
	}
	state, err := t.State()
	if err != nil {
		return false, err
	}
	if op == ast.ASTSequenceOpUnit {
		element, unit, argumentErr := state.manager.SequenceUnitArgument(expression)
		if argumentErr != nil || !unit {
			return false, argumentErr
		}
		return t.generatedSequenceTerm(element, visiting)
	}
	if op == ast.ASTSequenceOpConcat {
		left, right, concat, argumentErr := state.manager.SequenceConcatParts(expression)
		if argumentErr != nil || !concat {
			return false, argumentErr
		}
		leftGenerated, argumentErr := t.generatedSequenceTerm(left, visiting)
		if argumentErr != nil {
			return false, argumentErr
		}
		rightGenerated, argumentErr := t.generatedSequenceTerm(right, visiting)
		return leftGenerated || rightGenerated, argumentErr
	}
	return false, nil
}

func (t *SMTSequenceTheory) markUnsupported() error {
	if _, err := t.State(); err != nil {
		return err
	}
	t.unsupported++
	return nil
}

func (t *SMTSequenceTheory) ensureVariable(expression ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	count, err := state.manager.GetNumArgs(expression)
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		argument, argumentErr := state.manager.GetArg(expression, i)
		if argumentErr != nil {
			return argumentErr
		}
		if argumentErr = state.Internalize(argument, false); argumentErr != nil {
			return argumentErr
		}
	}
	node := state.context.enodes[expression]
	if node == nil {
		sort, sortErr := state.manager.GetSort(expression)
		if sortErr != nil {
			return sortErr
		}
		booleanSort, sortErr := state.manager.MkBoolSort()
		if sortErr != nil {
			return sortErr
		}
		isBoolean := sort == booleanSort
		if isBoolean {
			variable, found := state.BoolVar(expression)
			if !found {
				variable, sortErr = state.MkBoolVar(expression)
				if sortErr != nil {
					return sortErr
				}
			}
			data, dataErr := state.BooleanData(variable)
			if dataErr != nil {
				return dataErr
			}
			if !data.IsTheoryAtom() {
				if dataErr = state.SetVariableTheory(variable, t.FamilyID()); dataErr != nil {
					return dataErr
				}
			}
		}
		node, err = state.MkEnode(expression, false, isBoolean, true)
		if err != nil {
			return err
		}
	}
	sort, err := state.manager.GetSort(expression)
	if err != nil {
		return err
	}
	sequence, regex, err := t.sequenceSort(sort)
	if err != nil {
		return err
	}
	if !sequence && !regex {
		return nil
	}
	if !t.IsAttachedToVariable(node) {
		_, err = t.CreateVariable(node)
	}
	return err
}

func (t *SMTSequenceTheory) InternalizeAtom(expression ast.ASTHandle, _ bool) (bool, error) {
	return t.InternalizeTerm(expression)
}

func (t *SMTSequenceTheory) InternalizeTerm(expression ast.ASTHandle) (bool, error) {
	op, own, err := t.operation(expression)
	if err != nil || !own {
		return false, err
	}
	ground, err := t.addGroundOperationAxiom(expression, op)
	if err != nil {
		return false, err
	}
	regexReduced := false
	if op == 12 {
		regexReduced, err = t.addRegexMembershipAxiom(expression)
		if err != nil {
			return false, err
		}
		state, argumentErr := t.State()
		if argumentErr != nil {
			return false, argumentErr
		}
		regexExpression, argumentErr := state.manager.GetArg(expression, 1)
		if argumentErr != nil {
			return false, argumentErr
		}
		if regexReduced {
			t.trackInternalTerms(expression)
		}
		if argumentErr = t.trackRegexTree(regexExpression); argumentErr != nil {
			return false, argumentErr
		}
	}
	if err = t.ensureVariable(expression); err != nil {
		return false, err
	}
	if !ground {
		if op == 9 {
			err = t.addLengthAxiom(expression)
		} else if op == 8 {
			err = t.addAtAxioms(expression)
		} else if op == 6 {
			err = t.addExtractAxioms(expression)
		} else if op == 10 {
			err = t.addIndexAxioms(expression)
		} else if op == 7 {
			err = t.addReplaceAxioms(expression)
		} else if op == 27 {
			err = t.addIntToStringAxioms(expression)
		} else if op == 28 {
			err = t.addStringToIntAxioms(expression)
		}
		if err != nil {
			return false, err
		}
	}
	value, err := t.supportedValue(expression)
	if err != nil {
		return false, err
	}
	_, internal := t.internalTerms[expression]
	generated, err := t.generatedSequenceTerm(expression, make(map[ast.ASTHandle]struct{}))
	if err != nil {
		return false, err
	}
	implemented := op == ast.ASTSequenceOpUnit || op == ast.ASTSequenceOpConcat ||
		3 <= op && op <= 10 || op == 27 || op == 28
	if !value && !internal && !generated && !implemented && op != 12 {
		if err = t.markUnsupported(); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (*SMTSequenceTheory) InternalizeEquality(ast.ASTHandle, int32) error { return nil }

func (t *SMTSequenceTheory) ApplySortConstraint(node *SMTEnode, sort ast.ASTHandle) error {
	if node == nil {
		return ast.ErrASTDomain
	}
	sequence, regex, err := t.sequenceSort(sort)
	if err != nil || (!sequence && !regex) {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	if !t.IsAttachedToVariable(node) {
		if _, err = t.CreateVariable(node); err != nil {
			return err
		}
	}
	if regex {
		if _, exact := t.internalTerms[node.Owner()]; exact {
			return nil
		}
		return t.markUnsupported()
	}
	return nil
}

func (*SMTSequenceTheory) UseDisequalities() bool { return true }

func (t *SMTSequenceTheory) CanPropagate() bool {
	return t != nil && t.axioms != nil && t.axioms.Pending()
}

func (t *SMTSequenceTheory) Propagate() error {
	state, err := t.State()
	if err != nil {
		return err
	}
	for t.axioms.Pending() && !state.Inconsistent() {
		expression, frontErr := t.axioms.Front()
		if frontErr != nil {
			return frontErr
		}
		if frontErr = t.assertConstraint(expression); frontErr != nil {
			return frontErr
		}
		if frontErr = t.axioms.Advance(); frontErr != nil {
			return frontErr
		}
	}
	return nil
}

func (t *SMTSequenceTheory) equalityLiteral(left, right ast.ASTHandle) SMTLiteral {
	state, err := t.State()
	if err != nil {
		return SMTNullLiteral
	}
	for _, pair := range [][2]ast.ASTHandle{{left, right}, {right, left}} {
		equality, makeErr := state.manager.MkEq(pair[0], pair[1])
		if makeErr != nil {
			continue
		}
		if variable, found := state.BoolVar(equality); found {
			return NewSMTLiteral(variable, false)
		}
	}
	return SMTNullLiteral
}

func (t *SMTSequenceTheory) concreteDistinct(left, right ast.ASTHandle) (bool, bool, error) {
	state, err := t.State()
	if err != nil {
		return false, false, err
	}
	leftText, leftString, err := state.manager.SequenceStringValue(left)
	if err != nil {
		return false, false, err
	}
	rightText, rightString, err := state.manager.SequenceStringValue(right)
	if err != nil {
		return false, false, err
	}
	if leftString || rightString {
		return leftString && rightString && leftText != rightText, leftString && rightString, nil
	}
	leftEmpty, err := state.manager.IsSequenceEmpty(left)
	if err != nil {
		return false, false, err
	}
	rightEmpty, err := state.manager.IsSequenceEmpty(right)
	if err != nil {
		return false, false, err
	}
	return false, leftEmpty && rightEmpty, nil
}

func (t *SMTSequenceTheory) setDependencyConflict(dependency *smtSequenceDependency) error {
	literals, equalities := t.dependencyEvidence(dependency)
	if len(literals) == 0 && len(equalities) == 0 {
		return t.markUnsupported()
	}
	state, err := t.State()
	if err != nil {
		return err
	}
	// Source linearize refuses a dependency whose literals are no longer
	// asserted. Equality antecedents remain actual scoped enode evidence.
	for _, literal := range literals {
		value, err := state.Assignment(literal)
		if err != nil {
			return err
		}
		if value != lbool.True {
			return nil
		}
	}
	if len(equalities) == 0 {
		return state.SetTheoryConflict(t.FamilyID(), literals)
	}
	if state.Inconsistent() {
		return nil
	}
	owner, err := NewSMTExtTheoryConflictJustification(state.manager, t.FamilyID(), literals, equalities, nil)
	if err != nil {
		return err
	}
	wrapped, err := NewSMTBooleanJustification(owner)
	if err != nil {
		return errors.Join(err, owner.Release(state.manager))
	}
	state.inconsistent = true
	state.conflict = wrapped
	state.conflictLiteral = SMTNullLiteral
	state.statistics.NumConflicts++
	return nil
}

func (t *SMTSequenceTheory) NewEquality(leftVar, rightVar SMTTheoryVar) (result error) {
	leftNode, err := t.Enode(leftVar)
	if err != nil {
		return err
	}
	rightNode, err := t.Enode(rightVar)
	if err != nil {
		return err
	}
	literal := t.equalityLiteral(leftNode.Owner(), rightNode.Owner())
	dependency := t.dependencies.Leaf(smtSequenceAssumption{left: leftNode, right: rightNode, literal: literal})
	state, err := t.State()
	if err != nil {
		return err
	}
	leftSort, err := state.manager.GetSort(leftNode.Owner())
	if err != nil {
		return err
	}
	sequence, _, err := t.sequenceSort(leftSort)
	if err != nil {
		return err
	}
	if !sequence {
		return nil
	}
	defer func() {
		if result == nil {
			result = t.enforceSequenceLengthCoherence(leftNode, rightNode)
		}
	}()
	leftConcat, err := state.manager.IsSequenceConcat(leftNode.Owner())
	if err != nil {
		return err
	}
	rightConcat, err := state.manager.IsSequenceConcat(rightNode.Owner())
	if err != nil {
		return err
	}
	if sequence && leftNode != rightNode && (leftConcat || rightConcat) {
		leftParts, flattenErr := state.manager.SequenceFlattenConcat(leftNode.Owner())
		if flattenErr != nil {
			return flattenErr
		}
		rightParts, flattenErr := state.manager.SequenceFlattenConcat(rightNode.Owner())
		if flattenErr != nil {
			return flattenErr
		}
		if flattenErr = t.equations.Push(t.nextEquation, leftParts, rightParts, dependency); flattenErr != nil {
			return flattenErr
		}
		t.nextEquation++
		return nil
	}
	left, leftDependency, err := t.solutions.Find(leftNode.Owner())
	if err != nil {
		return err
	}
	right, rightDependency, err := t.solutions.Find(rightNode.Owner())
	if err != nil {
		return err
	}
	dependency = t.dependencies.Join(dependency, t.dependencies.Join(leftDependency, rightDependency))
	if left == right {
		return nil
	}
	_, leftUnit, err := state.manager.SequenceUnitArgument(left)
	if err != nil {
		return err
	}
	_, rightUnit, err := state.manager.SequenceUnitArgument(right)
	if err != nil {
		return err
	}
	if sequence && (leftUnit || rightUnit) {
		solved, err := t.solveUnitWord([]ast.ASTHandle{left}, []ast.ASTHandle{right}, dependency)
		if err != nil || solved {
			return err
		}
		leftParts, err := state.manager.SequenceFlattenConcat(left)
		if err != nil {
			return err
		}
		rightParts, err := state.manager.SequenceFlattenConcat(right)
		if err != nil {
			return err
		}
		if err = t.equations.Push(t.nextEquation, leftParts, rightParts, dependency); err != nil {
			return err
		}
		t.nextEquation++
		return nil
	}
	distinct, bothConcrete, err := t.concreteDistinct(left, right)
	if err != nil {
		return err
	}
	if distinct {
		return t.setDependencyConflict(dependency)
	}
	if bothConcrete {
		return nil
	}
	leftValue, err := t.supportedValue(left)
	if err != nil {
		return err
	}
	rightValue, err := t.supportedValue(right)
	if err != nil {
		return err
	}
	if leftValue {
		return t.solutions.Update(right, left, dependency)
	}
	if rightValue {
		return t.solutions.Update(left, right, dependency)
	}
	return t.solutions.Update(left, right, dependency)
}

func (t *SMTSequenceTheory) NewDisequality(leftVar, rightVar SMTTheoryVar) error {
	left, err := t.Enode(leftVar)
	if err != nil {
		return err
	}
	right, err := t.Enode(rightVar)
	if err != nil {
		return err
	}
	if err = t.exclusions.Update(left.Owner(), right.Owner()); err != nil {
		return err
	}
	literal := t.equalityLiteral(left.Owner(), right.Owner())
	if literal != SMTNullLiteral {
		literal = literal.Negated()
	}
	dependency := t.dependencies.Leaf(smtSequenceAssumption{left: left, right: right, literal: literal})
	return t.disequations.Push(left.Owner(), right.Owner(), dependency)
}

func (t *SMTSequenceTheory) Assign(variable int32, value bool) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	if variable < SMTFirstBoolVar || int(variable) >= len(state.boolExpr) {
		return ast.ErrASTDomain
	}
	expression := state.boolExpr[variable]
	op, own, err := t.operation(expression)
	if err != nil || !own {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	if op != 3 && op != 4 && op != 5 {
		return nil
	}
	literal := NewSMTLiteral(variable, !value)
	dependency := t.dependencies.Leaf(smtSequenceAssumption{literal: literal})
	if err = t.relations.Push(expression, value, dependency); err != nil {
		return err
	}
	if op == 5 && !value {
		done, err := t.canonizesNegativeSequence(expression)
		if err != nil || done {
			return err
		}
	}
	if err = t.addRelationAxioms(expression, op, value); err != nil {
		return err
	}
	if op == 5 && !value {
		return t.nonContains.Push(expression, dependency)
	}
	if op == 3 && !value {
		return t.addNegativePrefixStep(expression)
	}
	return nil
}

func (t *SMTSequenceTheory) PushScope() error {
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	if err := t.solutions.PushScope(); err != nil {
		_ = t.SMTTheoryBase.PopScopes(1)
		return err
	}
	if err := t.exclusions.PushScope(); err != nil {
		_ = t.solutions.PopScopes(1)
		_ = t.SMTTheoryBase.PopScopes(1)
		return err
	}
	if err := t.equations.PushScope(); err != nil {
		_ = t.exclusions.PopScopes(1)
		_ = t.solutions.PopScopes(1)
		_ = t.SMTTheoryBase.PopScopes(1)
		return err
	}
	if err := t.disequations.PushScope(); err != nil {
		_ = t.equations.PopScopes(1)
		_ = t.exclusions.PopScopes(1)
		_ = t.solutions.PopScopes(1)
		_ = t.SMTTheoryBase.PopScopes(1)
		return err
	}
	if err := t.nonContains.PushScope(); err != nil {
		_ = t.disequations.PopScopes(1)
		_ = t.equations.PopScopes(1)
		_ = t.exclusions.PopScopes(1)
		_ = t.solutions.PopScopes(1)
		_ = t.SMTTheoryBase.PopScopes(1)
		return err
	}
	if err := t.relations.PushScope(); err != nil {
		_ = t.nonContains.PopScopes(1)
		_ = t.disequations.PopScopes(1)
		_ = t.equations.PopScopes(1)
		_ = t.exclusions.PopScopes(1)
		_ = t.solutions.PopScopes(1)
		_ = t.SMTTheoryBase.PopScopes(1)
		return err
	}
	if err := t.axioms.PushScope(); err != nil {
		_ = t.relations.PopScopes(1)
		_ = t.nonContains.PopScopes(1)
		_ = t.disequations.PopScopes(1)
		_ = t.equations.PopScopes(1)
		_ = t.exclusions.PopScopes(1)
		_ = t.solutions.PopScopes(1)
		_ = t.SMTTheoryBase.PopScopes(1)
		return err
	}
	t.limits = append(t.limits, t.unsupported)
	t.internalLim = append(t.internalLim, len(t.internalLog))
	t.branchLim = append(t.branchLim, len(t.branchLog))
	t.lengthLim = append(t.lengthLim, len(t.lengthLog))
	return nil
}

func (t *SMTSequenceTheory) PopScopes(count uint32) error {
	if count == 0 || count > uint32(len(t.limits)) {
		return ast.ErrASTDomain
	}
	level := len(t.limits) - int(count)
	unsupported := t.limits[level]
	internalLimit := t.internalLim[level]
	if err := t.solutions.PopScopes(count); err != nil {
		return err
	}
	if err := t.exclusions.PopScopes(count); err != nil {
		return err
	}
	if err := t.equations.PopScopes(count); err != nil {
		return err
	}
	if err := t.disequations.PopScopes(count); err != nil {
		return err
	}
	if err := t.nonContains.PopScopes(count); err != nil {
		return err
	}
	if err := t.relations.PopScopes(count); err != nil {
		return err
	}
	if err := t.axioms.PopScopes(count); err != nil {
		return err
	}
	if err := t.SMTTheoryBase.PopScopes(count); err != nil {
		return err
	}
	t.unsupported = unsupported
	t.limits = t.limits[:level]
	for i := len(t.internalLog) - 1; i >= internalLimit; i-- {
		delete(t.internalTerms, t.internalLog[i])
	}
	t.internalLog = t.internalLog[:internalLimit]
	t.internalLim = t.internalLim[:level]
	for _, key := range t.branchLog[t.branchLim[level]:] {
		delete(t.branchStart, key)
	}
	t.branchLog = t.branchLog[:t.branchLim[level]]
	t.branchLim = t.branchLim[:level]
	for i := len(t.lengthLog) - 1; i >= t.lengthLim[level]; i-- {
		expression, err := t.state.manager.RefGet(t.lengthLog[i])
		if err != nil {
			return err
		}
		delete(t.lengths, expression)
		if err = t.state.manager.DelRef(&t.lengthLog[i]); err != nil {
			return err
		}
	}
	t.lengthLog = t.lengthLog[:t.lengthLim[level]]
	t.lengthLim = t.lengthLim[:level]
	return nil
}

func (t *SMTSequenceTheory) concreteString(expression ast.ASTHandle, dependency *smtSequenceDependency, visiting map[ast.ASTHandle]struct{}) (string, bool, *smtSequenceDependency, error) {
	state, err := t.State()
	if err != nil {
		return "", false, dependency, err
	}
	if _, found := visiting[expression]; found {
		return "", false, dependency, nil
	}
	visiting[expression] = struct{}{}
	defer delete(visiting, expression)
	representative, foundDependency, err := t.solutions.Find(expression)
	if err != nil {
		return "", false, dependency, err
	}
	dependency = t.dependencies.Join(dependency, foundDependency)
	if representative != expression {
		return t.concreteString(representative, dependency, visiting)
	}
	if text, matched, textErr := state.manager.SequenceStringValue(expression); textErr != nil || matched {
		return text, matched, dependency, textErr
	}
	if element, unit, unitErr := state.manager.SequenceUnitArgument(expression); unitErr != nil {
		return "", false, dependency, unitErr
	} else if unit {
		value, fixed, valueErr := state.manager.SequenceUnitByteElementValue(element)
		if valueErr != nil || fixed {
			return string([]byte{value}), fixed, dependency, valueErr
		}
		// Canonization of a unit also uses established character equalities.
		// Keep that equality evidence when the concrete word explains a
		// conflict, rather than reading a guessed model character.
		if node := state.context.enodes[element]; node != nil {
			for current := node.Next(); current != node; current = current.Next() {
				value, fixed, valueErr = state.manager.SequenceUnitByteElementValue(current.Owner())
				if valueErr != nil {
					return "", false, dependency, valueErr
				}
				if fixed {
					dependency = t.dependencies.Join(dependency, t.dependencies.Leaf(smtSequenceAssumption{left: node, right: current, literal: SMTNullLiteral}))
					return string([]byte{value}), true, dependency, nil
				}
			}
		}
		return "", false, dependency, nil
	}
	left, right, concat, err := state.manager.SequenceConcatParts(expression)
	if err != nil || !concat {
		return "", false, dependency, err
	}
	leftText, leftConcrete, dependency, err := t.concreteString(left, dependency, visiting)
	if err != nil || !leftConcrete {
		return "", false, dependency, err
	}
	rightText, rightConcrete, dependency, err := t.concreteString(right, dependency, visiting)
	if err != nil || !rightConcrete {
		return "", false, dependency, err
	}
	return leftText + rightText, true, dependency, nil
}

func (t *SMTSequenceTheory) concreteWord(parts []ast.ASTHandle, dependency *smtSequenceDependency) (string, bool, *smtSequenceDependency, error) {
	var result string
	for _, part := range parts {
		text, concrete, updated, err := t.concreteString(part, dependency, make(map[ast.ASTHandle]struct{}))
		if err != nil || !concrete {
			return "", false, updated, err
		}
		dependency = updated
		result += text
	}
	return result, true, dependency, nil
}

func (t *SMTSequenceTheory) checkConcreteEquations() (bool, error) {
	for index := 0; index < t.equations.Len(); index++ {
		_, left, right, dependency, err := t.equations.At(index)
		if err != nil {
			return false, err
		}
		leftText, leftConcrete, dependency, err := t.concreteWord(left, dependency)
		if err != nil {
			return false, err
		}
		rightText, rightConcrete, dependency, err := t.concreteWord(right, dependency)
		if err != nil {
			return false, err
		}
		if leftConcrete && rightConcrete && leftText != rightText {
			if err = t.setDependencyConflict(dependency); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}

func (t *SMTSequenceTheory) checkConcreteDisequations() (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
entries:
	for index := 0; index < t.disequations.Len(); index++ {
		left, right, _, _, literals, dependency, err := t.disequations.At(index)
		if err != nil {
			return false, err
		}
		for _, literal := range literals {
			value, err := state.Assignment(literal)
			if err != nil {
				return false, err
			}
			if value != lbool.True {
				continue entries
			}
			dependency = t.dependencies.Join(dependency, t.dependencies.Leaf(smtSequenceAssumption{literal: literal}))
		}
		leftText, leftConcrete, dependency, err := t.concreteString(left, dependency, make(map[ast.ASTHandle]struct{}))
		if err != nil {
			return false, err
		}
		rightText, rightConcrete, dependency, err := t.concreteString(right, dependency, make(map[ast.ASTHandle]struct{}))
		if err != nil {
			return false, err
		}
		if leftConcrete && rightConcrete && leftText == rightText {
			if err = t.setDependencyConflict(dependency); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}

func (t *SMTSequenceTheory) checkConcreteRelations() (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	for index := 0; index < t.relations.Len(); index++ {
		expression, asserted, dependency, entryErr := t.relations.At(index)
		if entryErr != nil {
			return false, entryErr
		}
		op, own, operationErr := t.operation(expression)
		if operationErr != nil || !own || op < 3 || op > 5 {
			if operationErr != nil {
				return false, operationErr
			}
			return false, ast.ErrASTDomain
		}
		left, right, matched, partsErr := state.manager.SequenceBinaryOperationArguments(expression, op)
		if partsErr != nil || !matched {
			if partsErr != nil {
				return false, partsErr
			}
			return false, ast.ErrASTDomain
		}
		leftText, leftConcrete, dependency, entryErr := t.concreteString(left, dependency, make(map[ast.ASTHandle]struct{}))
		if entryErr != nil {
			return false, entryErr
		}
		rightText, rightConcrete, dependency, entryErr := t.concreteString(right, dependency, make(map[ast.ASTHandle]struct{}))
		if entryErr != nil {
			return false, entryErr
		}
		if !leftConcrete || !rightConcrete {
			continue
		}
		actual := false
		switch op {
		case 3:
			actual = strings.HasPrefix(rightText, leftText)
		case 4:
			actual = strings.HasSuffix(rightText, leftText)
		case 5:
			actual = strings.Contains(leftText, rightText)
		}
		if actual != asserted {
			if entryErr = t.setDependencyConflict(dependency); entryErr != nil {
				return false, entryErr
			}
			return true, nil
		}
	}
	return false, nil
}

func (t *SMTSequenceTheory) FinalCheck() (SMTFinalCheckStatus, error) {
	state, err := t.State()
	if err != nil {
		return SMTFinalCheckGiveUp, fmt.Errorf("sequence final state: %w", err)
	}
	propagateAxioms := func(reported bool) (bool, error) {
		if !reported {
			return false, nil
		}
		before := state.searchProgress()
		if propagateErr := t.Propagate(); propagateErr != nil {
			return false, propagateErr
		}
		return state.Inconsistent() || state.searchProgress() != before, nil
	}
	// Native final_check_eh simplifies equations before reducing m_ncs.
	t.branchProgress = false
	progress, err := t.solveWordEquations()
	if err != nil {
		return SMTFinalCheckGiveUp, fmt.Errorf("sequence word equations: %w", err)
	}
	if progress {
		if !t.branchProgress {
			t.statistics.solveEqualities++
		}
		return SMTFinalCheckContinue, nil
	}
	progress, err = t.addSolvedTailEmptyMaximum()
	if err != nil {
		return SMTFinalCheckGiveUp, fmt.Errorf("sequence tail maximum length: %w", err)
	}
	if progress, err = propagateAxioms(progress); err != nil {
		return SMTFinalCheckGiveUp, err
	} else if progress {
		return SMTFinalCheckContinue, nil
	}
	progress, err = t.addIntegerStringLengthRefinement()
	if err != nil {
		return SMTFinalCheckGiveUp, fmt.Errorf("sequence integer-string length: %w", err)
	}
	if progress, err = propagateAxioms(progress); err != nil {
		return SMTFinalCheckGiveUp, err
	} else if progress {
		t.statistics.integerString++
		return SMTFinalCheckContinue, nil
	}
	regexProgress, regexUnresolved, err := t.reduceExpandedRegexMemberships()
	if err != nil {
		return SMTFinalCheckGiveUp, err
	}
	reportedRegexProgress := regexProgress
	if regexProgress, err = propagateAxioms(reportedRegexProgress); err != nil {
		return SMTFinalCheckGiveUp, fmt.Errorf("sequence regex propagation: %w", err)
	} else if regexProgress || reportedRegexProgress {
		t.statistics.propagateAutomata++
		if !regexProgress {
			// Source m_new_propagation includes a newly recorded regex
			// refinement even when its clauses are already satisfied. The
			// next final check must inspect the remaining word equations.
			state.theoryProgress++
		}
		return SMTFinalCheckContinue, nil
	}
	progress, err = t.addLengthLowerBoundDecomposition()
	if err != nil {
		return SMTFinalCheckGiveUp, fmt.Errorf("sequence length lower bound: %w", err)
	}
	if progress, err = propagateAxioms(progress); err != nil {
		return SMTFinalCheckGiveUp, err
	} else if progress {
		t.statistics.checkLengthCoherence++
		return SMTFinalCheckContinue, nil
	}
	progress, err = t.addFixedLengthDecomposition()
	if err != nil {
		return SMTFinalCheckGiveUp, fmt.Errorf("sequence fixed length: %w", err)
	}
	if progress, err = propagateAxioms(progress); err != nil {
		return SMTFinalCheckGiveUp, err
	} else if progress {
		t.statistics.fixedLength++
		return SMTFinalCheckContinue, nil
	}
	progress, err = t.addFixedLengthDisequalityReduction()
	if err != nil {
		return SMTFinalCheckGiveUp, fmt.Errorf("sequence fixed disequality: %w", err)
	}
	if progress, err = propagateAxioms(progress); err != nil {
		return SMTFinalCheckGiveUp, err
	} else if progress {
		t.statistics.solveDisequalities++
		return SMTFinalCheckContinue, nil
	}
	conflict, err := t.checkConcreteEquations()
	if err != nil {
		return SMTFinalCheckGiveUp, fmt.Errorf("sequence concrete equations: %w", err)
	}
	if conflict {
		t.statistics.solveEqualities++
		return SMTFinalCheckContinue, nil
	}
	conflict, err = t.checkConcreteRelations()
	if err != nil {
		return SMTFinalCheckGiveUp, fmt.Errorf("sequence concrete relations: %w", err)
	}
	if conflict {
		t.statistics.propagateContains++
		return SMTFinalCheckContinue, nil
	}
	conflict, err = t.checkNegativeContains()
	if err != nil {
		return SMTFinalCheckGiveUp, fmt.Errorf("sequence negative containment: %w", err)
	}
	if conflict {
		return SMTFinalCheckContinue, nil
	}
	beforeRetries := state.searchProgress()
	for i := 0; i < t.relations.Len(); i++ {
		expression, value, _, err := t.relations.At(i)
		if err != nil {
			return SMTFinalCheckGiveUp, err
		}
		operation, own, err := t.operation(expression)
		if err != nil {
			return SMTFinalCheckGiveUp, err
		}
		if !value && own && operation == 3 {
			if err = t.addNegativePrefixStep(expression); err != nil {
				return SMTFinalCheckGiveUp, err
			}
		}
	}
	if t.axioms.Pending() {
		progress, err = propagateAxioms(true)
		if err != nil {
			return SMTFinalCheckGiveUp, err
		}
		if progress {
			return SMTFinalCheckContinue, nil
		}
	}
	if state.searchProgress() != beforeRetries {
		return SMTFinalCheckContinue, nil
	}
	t.branchProgress = false
	progress, err = t.solveWordEquations()
	if err != nil {
		return SMTFinalCheckGiveUp, fmt.Errorf("sequence word equations: %w", err)
	}
	if progress {
		if !t.branchProgress {
			t.statistics.solveEqualities++
		}
		return SMTFinalCheckContinue, nil
	}
	conflict, err = t.checkConcreteDisequations()
	if err != nil {
		return SMTFinalCheckGiveUp, fmt.Errorf("sequence concrete disequations: %w", err)
	}
	if conflict {
		t.statistics.solveDisequalities++
		return SMTFinalCheckContinue, nil
	}
	conflict, err = t.checkReducedDisequations()
	if err != nil {
		return SMTFinalCheckGiveUp, fmt.Errorf("sequence reduced disequations: %w", err)
	}
	if conflict {
		t.statistics.solveDisequalities++
		return SMTFinalCheckContinue, nil
	}
	progress, err = t.checkTrackedSequenceLengthCoherence()
	if err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if progress {
		t.statistics.checkLengthCoherence++
		if t.axioms.Pending() {
			if err = t.Propagate(); err != nil {
				return SMTFinalCheckGiveUp, err
			}
		}
		return SMTFinalCheckContinue, nil
	}
	progress, err = t.checkSequenceExtensionality()
	if err != nil {
		return SMTFinalCheckGiveUp, fmt.Errorf("sequence extensionality: %w", err)
	}
	if progress {
		t.statistics.extensionality++
		return SMTFinalCheckContinue, nil
	}
	unresolvedEquation, err := t.hasUnresolvedWordEquation()
	if err != nil {
		return SMTFinalCheckGiveUp, fmt.Errorf("sequence solved equation audit: %w", err)
	}
	if unresolvedEquation || t.nonContains.Len() != 0 || t.unsupported != 0 || regexUnresolved {
		return SMTFinalCheckGiveUp, nil
	}
	return SMTFinalCheckDone, nil
}

func (*SMTSequenceTheory) Name() string { return "seq" }

func (t *SMTSequenceTheory) InitializeModel(generator *SMTModelGenerator) error {
	state, err := t.State()
	if err != nil || generator == nil || generator.model == nil || generator.manager != state.manager {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	factory, err := protomodel.NewSequenceFactory(generator.model, t.FamilyID())
	if err != nil {
		return err
	}
	if err = generator.RegisterFactory(factory); err != nil {
		return err
	}
	bitVectorFamily, err := state.manager.BitVectorUtilityFamilyID()
	if err != nil {
		return err
	}
	if generator.model.Factory(bitVectorFamily) != nil {
		return nil
	}
	bitVectorFactory, err := protomodel.NewBitVectorFactory(state.manager)
	if err != nil {
		return err
	}
	return generator.RegisterFactory(bitVectorFactory)
}

func (t *SMTSequenceTheory) ModelValue(node *SMTEnode, generator *SMTModelGenerator) (SMTModelValueProcedure, error) {
	if node == nil || generator == nil || generator.model == nil || !t.IsAttachedToVariable(node) {
		return nil, ast.ErrASTDomain
	}
	state, err := t.State()
	if err != nil || generator.manager != state.manager {
		if err != nil {
			return nil, err
		}
		return nil, ast.ErrASTDomain
	}
	root := node.Root()
	for current := root; ; current = current.Next() {
		if value, valueErr := t.supportedValue(current.Owner()); valueErr != nil {
			return nil, valueErr
		} else if value {
			return NewSMTExpressionValueProcedure(current.Owner())
		}
		if current.Next() == root {
			break
		}
	}
	for current := root; ; current = current.Next() {
		procedure, structured, procedureErr := t.structuredModelValue(current.Owner(), root, generator)
		if procedureErr != nil {
			return nil, procedureErr
		}
		if structured {
			return procedure, nil
		}
		if current.Next() == root {
			break
		}
	}
	sort, err := state.manager.GetSort(node.Owner())
	if err != nil {
		return nil, err
	}
	value, err := generator.model.FreshValue(sort)
	if err != nil {
		return nil, err
	}
	return NewSMTExpressionValueProcedure(value)
}

func (t *SMTSequenceTheory) Close() (result error) {
	if t == nil || t.solutions == nil || t.exclusions == nil || t.equations == nil || t.disequations == nil || t.nonContains == nil || t.relations == nil || t.axioms == nil {
		return ast.ErrASTHandle
	}
	result = errors.Join(t.solutions.Close(), t.exclusions.Close(), t.equations.Close(), t.disequations.Close(), t.nonContains.Close(), t.relations.Close(), t.axioms.Close())
	for i := len(t.lengthLog) - 1; i >= 0; i-- {
		result = errors.Join(result, t.state.manager.DelRef(&t.lengthLog[i]))
	}
	t.lengths = nil
	t.lengthLog = nil
	t.lengthLim = nil
	if t.rewrite != nil {
		result = errors.Join(result, t.rewrite.close())
		t.rewrite = nil
	}
	t.solutions = nil
	t.exclusions = nil
	t.equations = nil
	t.disequations = nil
	t.nonContains = nil
	t.relations = nil
	t.axioms = nil
	t.dependencies = nil
	t.internalTerms = nil
	t.internalLog = nil
	t.internalLim = nil
	t.branchStart = nil
	t.branchLog = nil
	t.branchLim = nil
	t.limits = nil
	if result != nil {
		return fmt.Errorf("sequence theory state: %w", result)
	}
	return nil
}

var _ SMTTheoryInternalizer = (*SMTSequenceTheory)(nil)
var _ SMTModelValueTheory = (*SMTSequenceTheory)(nil)
