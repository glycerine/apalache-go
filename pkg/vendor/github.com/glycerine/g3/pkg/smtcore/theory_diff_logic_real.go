package smtcore

// Source: retained real fragment of src/smt/theory_diff_logic.h and
// src/smt/theory_diff_logic_def.h.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
)

// SMTRealDiffTheory reuses the arithmetic syntax decomposition from the
// integer specialization and owns the concrete infinitesimal graph required
// for strict real complements.
type SMTRealDiffTheory struct {
	*SMTIntegerDiffTheory
	mixed             bool
	realGraph         *SMTInfRationalDiffGraph
	realZero          SMTTheoryVar
	realAtoms         []smtIntegerDiffAtom
	realAtomByBool    map[int32]int
	realAssertions    []smtIntegerDiffAssertion
	realAssertionHead int
	realScopes        []smtIntegerDiffScope
	realNonDifference bool
	delta             big.Rat
	realStatistics    smtDiffStatistics
}

func NewSMTRealDiffTheory(family ast.FamilyID) (*SMTRealDiffTheory, error) {
	syntax, err := NewSMTIntegerDiffTheory(family)
	if err != nil {
		return nil, err
	}
	return &SMTRealDiffTheory{
		SMTIntegerDiffTheory: syntax,
		realGraph:            NewSMTInfRationalDiffGraph(),
		realZero:             SMTNullTheoryVar,
		realAtomByBool:       make(map[int32]int),
	}, nil
}

// The concrete mi_ext graph uses the same infinitesimal rational values, but
// integer atoms use a unit strict-complement step and publish Int numerals.
func NewSMTMixedDiffTheory(family ast.FamilyID) (*SMTRealDiffTheory, error) {
	t, err := NewSMTRealDiffTheory(family)
	if err == nil {
		t.mixed = true
	}
	return t, err
}

func (t *SMTRealDiffTheory) Initialize(state *SMTInternalizerState) error {
	if t == nil || t.SMTTheoryBase == nil || t.realGraph == nil {
		return ast.ErrASTHandle
	}
	if err := t.SMTTheoryBase.Initialize(state); err != nil {
		return err
	}
	zero, err := state.manager.ArithmeticNumeralRat(new(big.Rat), false)
	if err != nil {
		return err
	}
	node := state.context.enodes[zero]
	if node == nil {
		node, err = state.MkEnode(zero, false, false, true)
		if err != nil {
			return err
		}
	}
	t.realZero, err = t.createRealVariable(node)
	return err
}

func (t *SMTRealDiffTheory) createRealVariable(node *SMTEnode) (SMTTheoryVar, error) {
	if t.IsAttachedToVariable(node) {
		return node.TheoryVar(t.FamilyID()), nil
	}
	variable, err := t.SMTTheoryBase.CreateVariable(node)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if err = t.realGraph.InitVariable(int32(variable)); err != nil {
		return SMTNullTheoryVar, err
	}
	return variable, nil
}

func (t *SMTRealDiffTheory) realSort(sort ast.ASTHandle) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	family, err := state.manager.GetFamilyID(sort)
	if err != nil || family != t.FamilyID() {
		return false, err
	}
	kind, err := state.manager.GetDeclKind(sort)
	return kind == 0, err
}

func (t *SMTRealDiffTheory) ApplySortConstraint(node *SMTEnode, sort ast.ASTHandle) error {
	real, err := t.realSort(sort)
	if err != nil {
		return err
	}
	if !real {
		integer, err := t.integerSort(sort)
		if err != nil {
			return err
		}
		if !t.mixed || !integer {
			t.realNonDifference = true
			return nil
		}
	}
	_, err = t.createRealVariable(node)
	return err
}

func finiteSMTInfRational(value *big.Rat) smtInfRational {
	return newSMTInfRational(value, nil)
}

func (t *SMTRealDiffTheory) addRealFixedValue(variable SMTTheoryVar, value *big.Rat) error {
	forwardWeight := finiteSMTInfRational(value)
	forward, err := t.realGraph.AddEdge(int32(t.realZero), int32(variable), &forwardWeight, SMTNullLiteral)
	if err != nil {
		return err
	}
	if feasible, enableErr := t.realGraph.EnableEdge(forward); enableErr != nil || !feasible {
		if enableErr != nil {
			return enableErr
		}
		return ast.ErrASTDomain
	}
	negative := new(big.Rat).Neg(value)
	backwardWeight := finiteSMTInfRational(negative)
	backward, err := t.realGraph.AddEdge(int32(variable), int32(t.realZero), &backwardWeight, SMTNullLiteral)
	if err != nil {
		return err
	}
	if feasible, enableErr := t.realGraph.EnableEdge(backward); enableErr != nil || !feasible {
		if enableErr != nil {
			return enableErr
		}
		return ast.ErrASTDomain
	}
	return nil
}

func (t *SMTRealDiffTheory) addRealOffset(source, target SMTTheoryVar, value *big.Rat) error {
	forwardWeight := finiteSMTInfRational(value)
	forward, err := t.realGraph.AddEdge(int32(source), int32(target), &forwardWeight, SMTNullLiteral)
	if err != nil {
		return err
	}
	if feasible, enableErr := t.realGraph.EnableEdge(forward); enableErr != nil || !feasible {
		if enableErr != nil {
			return enableErr
		}
		return ast.ErrASTDomain
	}
	negative := new(big.Rat).Neg(value)
	backwardWeight := finiteSMTInfRational(negative)
	backward, err := t.realGraph.AddEdge(int32(target), int32(source), &backwardWeight, SMTNullLiteral)
	if err != nil {
		return err
	}
	if feasible, enableErr := t.realGraph.EnableEdge(backward); enableErr != nil || !feasible {
		if enableErr != nil {
			return enableErr
		}
		return ast.ErrASTDomain
	}
	return nil
}

func (t *SMTRealDiffTheory) addRealBoundAxioms(source, target SMTTheoryVar, weight *smtInfRational, literal SMTLiteral) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	equal, lower, upper := SMTNullLiteral, SMTNullLiteral, SMTNullLiteral
	var lowerWeight, upperWeight smtInfRational
	for _, edgeID := range t.realGraph.outEdges[source] {
		edge := &t.realGraph.edges[edgeID]
		if edge.target != int32(target) || edge.explanation == SMTNullLiteral {
			continue
		}
		switch edge.weight.Cmp(weight) {
		case 0:
			equal = edge.explanation
		case -1:
			if lower == SMTNullLiteral || edge.weight.Cmp(&lowerWeight) > 0 {
				lower, lowerWeight = edge.explanation, edge.weight
			}
		case 1:
			if upper == SMTNullLiteral || edge.weight.Cmp(&upperWeight) < 0 {
				upper, upperWeight = edge.explanation, edge.weight
			}
		}
	}
	if equal != SMTNullLiteral {
		if err = state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{equal.Negated(), literal}); err != nil {
			return err
		}
		return state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{literal.Negated(), equal})
	}
	if lower != SMTNullLiteral {
		if err = state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{lower.Negated(), literal}); err != nil {
			return err
		}
	}
	if upper != SMTNullLiteral {
		return state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{literal.Negated(), upper})
	}
	return nil
}

func (t *SMTRealDiffTheory) InternalizeTerm(expression ast.ASTHandle) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	if node := state.context.enodes[expression]; node != nil {
		if !t.IsAttachedToVariable(node) {
			_, err = t.createRealVariable(node)
		}
		return err == nil, err
	}
	value, numeral, err := t.numeral(expression)
	if err != nil {
		return false, err
	}
	if numeral {
		if value.Sign() == 0 && !t.mixed {
			return true, nil
		}
		node, mkErr := state.MkEnode(expression, false, false, true)
		if mkErr != nil {
			return false, mkErr
		}
		variable, mkErr := t.createRealVariable(node)
		if mkErr != nil {
			return false, mkErr
		}
		return true, t.addRealFixedValue(variable, value)
	}
	base, value, offset, err := t.offset(expression)
	if err != nil {
		return false, err
	}
	if offset {
		source, sourceErr := t.ensureRealVariable(base)
		if sourceErr != nil {
			return false, sourceErr
		}
		count, countErr := state.manager.GetNumArgs(expression)
		if countErr != nil {
			return false, countErr
		}
		for index := uint32(0); index < count; index++ {
			argument, argumentErr := state.manager.GetArg(expression, index)
			if argumentErr != nil {
				return false, argumentErr
			}
			if internalizeErr := state.Internalize(argument, false); internalizeErr != nil {
				return false, internalizeErr
			}
		}
		node, mkErr := state.MkEnode(expression, false, false, true)
		if mkErr != nil {
			return false, mkErr
		}
		target, mkErr := t.createRealVariable(node)
		if mkErr != nil {
			return false, mkErr
		}
		return true, t.addRealOffset(source, target, value)
	}
	_, arithmetic, err := t.operation(expression)
	if err != nil {
		return false, err
	}
	if arithmetic {
		t.realNonDifference = true
		return false, nil
	}
	return false, nil
}

func (t *SMTRealDiffTheory) ensureRealVariable(expression ast.ASTHandle) (SMTTheoryVar, error) {
	state, err := t.State()
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if node := state.context.enodes[expression]; node != nil && t.IsAttachedToVariable(node) {
		return node.TheoryVar(t.FamilyID()), nil
	}
	if err = state.Internalize(expression, false); err != nil {
		return SMTNullTheoryVar, err
	}
	node := state.context.enodes[expression]
	if node == nil {
		if value, numeral, numeralErr := t.numeral(expression); numeralErr == nil && numeral && value.Sign() == 0 {
			return t.realZero, nil
		}
		return SMTNullTheoryVar, ast.ErrASTDomain
	}
	if t.IsAttachedToVariable(node) {
		return node.TheoryVar(t.FamilyID()), nil
	}
	t.realNonDifference = true
	return t.createRealVariable(node)
}

func (t *SMTRealDiffTheory) InternalizeAtom(expression ast.ASTHandle, _ bool) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	op, arithmetic, err := t.operation(expression)
	if err != nil {
		return false, err
	}
	if !arithmetic || op != smtArithOpLE && op != smtArithOpGE {
		t.realNonDifference = true
		return false, nil
	}
	lhs, err := state.manager.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	rhs, err := state.manager.GetArg(expression, 1)
	if err != nil {
		return false, err
	}
	isGE := op == smtArithOpGE
	bound, numeral, err := t.numeral(rhs)
	if err != nil {
		return false, err
	}
	if !numeral {
		lhs, rhs = rhs, lhs
		isGE = !isGE
		bound, numeral, err = t.numeral(rhs)
		if err != nil {
			return false, err
		}
	}
	if !numeral {
		t.realNonDifference = true
		return false, nil
	}
	terms, signs, err := t.decompose(lhs)
	if err != nil {
		return false, err
	}
	var source, target SMTTheoryVar
	if len(terms) == 2 && signs[0] != signs[1] {
		target, err = t.ensureRealVariable(terms[0])
		if err == nil {
			source, err = t.ensureRealVariable(terms[1])
		}
		if err != nil {
			return false, err
		}
		if !signs[0] {
			target, source = source, target
		}
	} else {
		target, err = t.ensureRealVariable(lhs)
		if err != nil {
			return false, err
		}
		source = t.realZero
	}
	weight := finiteSMTInfRational(bound)
	if isGE {
		target, source = source, target
		weight.Neg(&weight)
	}
	variable, err := state.MkBoolVar(expression)
	if err != nil {
		return false, err
	}
	if err = state.SetVariableTheory(variable, t.FamilyID()); err != nil {
		return false, err
	}
	literal := NewSMTLiteral(variable, false)
	if err = t.addRealBoundAxioms(source, target, &weight, literal); err != nil {
		return false, err
	}
	positive, err := t.realGraph.AddEdge(int32(source), int32(target), &weight, literal)
	if err != nil {
		return false, err
	}
	var negativeWeight smtInfRational
	negativeWeight.Neg(&weight)
	integer := false
	if t.mixed {
		sort, err := state.manager.GetSort(lhs)
		if err != nil {
			return false, err
		}
		integer, err = t.integerSort(sort)
		if err != nil {
			return false, err
		}
	}
	if integer {
		negativeWeight.rational.Sub(&negativeWeight.rational, big.NewRat(1, 1))
	} else {
		negativeWeight.infinitesimal.Sub(&negativeWeight.infinitesimal, big.NewRat(1, 1))
	}
	negative, err := t.realGraph.AddEdge(int32(target), int32(source), &negativeWeight, literal.Negated())
	if err != nil {
		return false, err
	}
	index := len(t.realAtoms)
	t.realAtoms = append(t.realAtoms, smtIntegerDiffAtom{variable: variable, positive: positive, negative: negative})
	t.realAtomByBool[variable] = index
	return true, nil
}

func (t *SMTRealDiffTheory) Assign(variable int32, truth bool) error {
	index, found := t.realAtomByBool[variable]
	if !found {
		return ast.ErrASTDomain
	}
	t.realAtoms[index].truth = truth
	t.realAssertions = append(t.realAssertions, smtIntegerDiffAssertion{atom: index, truth: truth})
	t.realStatistics.assertions++
	return nil
}

func (t *SMTRealDiffTheory) CanPropagate() bool {
	return t != nil && t.realAssertionHead < len(t.realAssertions)
}

func (t *SMTRealDiffTheory) Propagate() error {
	state, err := t.State()
	if err != nil {
		return err
	}
	for !state.inconsistent && t.CanPropagate() {
		assertion := t.realAssertions[t.realAssertionHead]
		t.realAssertionHead++
		atom := &t.realAtoms[assertion.atom]
		edge := atom.negative
		if assertion.truth {
			edge = atom.positive
		}
		feasible, enableErr := t.realGraph.EnableEdge(edge)
		if enableErr != nil {
			return enableErr
		}
		if !feasible {
			literals, explainErr := t.realGraph.NegativeCycleExplanations()
			if explainErr != nil {
				return explainErr
			}
			kept := literals[:0]
			for _, literal := range literals {
				if literal != SMTNullLiteral {
					kept = append(kept, literal)
				}
			}
			return t.setNegativeCycleConflict(kept)
		}
	}
	return nil
}

func (t *SMTRealDiffTheory) PushScope() error {
	if t == nil || t.SMTTheoryBase == nil || t.realGraph == nil {
		return ast.ErrASTHandle
	}
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	if err := t.realGraph.Push(); err != nil {
		_ = t.SMTTheoryBase.PopScopes(1)
		return err
	}
	if err := t.equalityAdapter.PushScope(); err != nil {
		_ = t.realGraph.Pop(1)
		_ = t.SMTTheoryBase.PopScopes(1)
		return err
	}
	t.realScopes = append(t.realScopes, smtIntegerDiffScope{len(t.realAtoms), len(t.realAssertions), t.realAssertionHead, t.realNonDifference})
	return nil
}

func (t *SMTRealDiffTheory) PopScopes(count uint32) error {
	if t == nil || count == 0 || int(count) > len(t.realScopes) {
		return ast.ErrASTDomain
	}
	level := len(t.realScopes) - int(count)
	scope := t.realScopes[level]
	for index := len(t.realAtoms); index > scope.atoms; index-- {
		delete(t.realAtomByBool, t.realAtoms[index-1].variable)
	}
	t.realAtoms = t.realAtoms[:scope.atoms]
	t.realAssertions = t.realAssertions[:scope.assertions]
	t.realAssertionHead = scope.head
	t.realNonDifference = scope.nonDifference
	t.realScopes = t.realScopes[:level]
	if err := t.equalityAdapter.PopScopes(count); err != nil {
		return err
	}
	oldEdges := t.realGraph.NumEdges()
	if err := t.realGraph.Pop(count); err != nil {
		return err
	}
	if oldEdges != t.realGraph.NumEdges() {
		t.optimization.resetSimplex()
	}
	return t.SMTTheoryBase.PopScopes(count)
}

func (t *SMTRealDiffTheory) FinalCheck() (SMTFinalCheckStatus, error) {
	if t.CanPropagate() {
		if err := t.Propagate(); err != nil {
			return SMTFinalCheckGiveUp, err
		}
		return SMTFinalCheckContinue, nil
	}
	if err := t.realGraph.SetToZero(int32(t.realZero)); err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if t.realNonDifference {
		return SMTFinalCheckGiveUp, nil
	}
	if t.mixed {
		t.computeModelDelta()
		for v, node := range t.variables {
			sort, err := t.state.manager.GetSort(node.owner)
			if err != nil {
				return SMTFinalCheckGiveUp, err
			}
			integer, err := t.integerSort(sort)
			if err != nil {
				return SMTFinalCheckGiveUp, err
			}
			if integer {
				var scaled, value big.Rat
				scaled.Mul(&t.delta, &t.realGraph.assignment[v].infinitesimal)
				value.Add(&t.realGraph.assignment[v].rational, &scaled)
				// The source rejects a fractional arithmetic-factory Int value.
				// Detect that model failure before publication.
				if !value.IsInt() {
					return SMTFinalCheckGiveUp, nil
				}
			}
		}
	}
	return SMTFinalCheckDone, nil
}

func (t *SMTRealDiffTheory) InitializeSearch() error {
	return t.equalityAdapter.InitializeSearch()
}

func (t *SMTRealDiffTheory) Restart() error {
	return t.equalityAdapter.Restart()
}

func (t *SMTRealDiffTheory) InternalizeEquality(expression ast.ASTHandle, variable int32) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	left, err := state.manager.GetArg(expression, 0)
	if err != nil {
		return err
	}
	if t.mixed {
		sort, err := state.manager.GetSort(left)
		if err != nil {
			return err
		}
		integer, err := t.integerSort(sort)
		if err != nil {
			return err
		}
		if integer {
			return t.SMTIntegerDiffTheory.InternalizeEquality(expression, variable)
		}
	}
	right, err := state.manager.GetArg(expression, 1)
	if err != nil {
		return err
	}
	minusOne, err := state.manager.ArithmeticNumeralRat(big.NewRat(-1, 1), false)
	if err != nil {
		return err
	}
	negativeRight, err := state.manager.MkFamilyApp(t.FamilyID(), smtArithOpMul, nil, []ast.ASTHandle{minusOne, right})
	if err != nil {
		return err
	}
	difference, err := state.manager.MkFamilyApp(t.FamilyID(), smtArithOpAdd, nil, []ast.ASTHandle{left, negativeRight})
	if err != nil {
		return err
	}
	zero, err := state.manager.ArithmeticNumeralRat(new(big.Rat), false)
	if err != nil {
		return err
	}
	lower, err := state.manager.MkFamilyApp(t.FamilyID(), smtArithOpLE, nil, []ast.ASTHandle{difference, zero})
	if err != nil {
		return err
	}
	upper, err := state.manager.MkFamilyApp(t.FamilyID(), smtArithOpGE, nil, []ast.ASTHandle{difference, zero})
	if err != nil {
		return err
	}
	if err = state.Internalize(lower, true); err != nil {
		return err
	}
	if err = state.Internalize(upper, true); err != nil {
		return err
	}
	lowerLiteral, err := state.Literal(lower)
	if err != nil {
		return err
	}
	upperLiteral, err := state.Literal(upper)
	if err != nil {
		return err
	}
	equality := NewSMTLiteral(variable, false)
	if err = state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{equality.Negated(), lowerLiteral}); err != nil {
		return err
	}
	if err = state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{equality.Negated(), upperLiteral}); err != nil {
		return err
	}
	if err = state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{equality, lowerLiteral.Negated(), upperLiteral.Negated()}); err != nil {
		return err
	}
	if t.addBinaryBounds {
		if err = state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{lowerLiteral, upperLiteral}); err != nil {
			return err
		}
	}
	return t.equalityAdapter.AddRelevancy(left, right, expression, lower, upper)
}

func (t *SMTRealDiffTheory) NewEquality(left, right SMTTheoryVar) error {
	return t.equalityAdapter.NewEquality(left, right)
}
func (t *SMTRealDiffTheory) NewDisequality(left, right SMTTheoryVar) error {
	return t.equalityAdapter.NewDisequality(left, right)
}

func (t *SMTRealDiffTheory) Graph() *SMTInfRationalDiffGraph {
	if t == nil {
		return nil
	}
	return t.realGraph
}

func (t *SMTRealDiffTheory) Name() string {
	if t.mixed {
		return "mixed-difference-logic"
	}
	return "real-difference-logic"
}

func (t *SMTRealDiffTheory) InitializeModel(generator *SMTModelGenerator) error {
	if t == nil || generator == nil {
		return ast.ErrASTHandle
	}
	state, err := t.State()
	if err != nil {
		return err
	}
	if generator.manager != state.manager {
		return ast.ErrASTDomain
	}
	t.computeModelDelta()
	factory, err := protomodel.NewArithmeticFactory(state.manager)
	if err != nil {
		return err
	}
	return generator.RegisterFactory(factory)
}

func (t *SMTRealDiffTheory) ModelValue(node *SMTEnode, generator *SMTModelGenerator) (SMTModelValueProcedure, error) {
	if t == nil || generator == nil || node == nil || !t.IsAttachedToVariable(node) {
		return nil, ast.ErrASTDomain
	}
	state, err := t.State()
	if err != nil {
		return nil, err
	}
	if generator.manager != state.manager {
		return nil, ast.ErrASTDomain
	}
	variable := node.TheoryVar(t.FamilyID())
	assignment, err := t.realGraph.Assignment(int32(variable))
	if err != nil {
		return nil, err
	}
	var scaled, value big.Rat
	scaled.Mul(&t.delta, &assignment.infinitesimal)
	value.Add(&assignment.rational, &scaled)
	integer := false
	if t.mixed {
		sort, err := state.manager.GetSort(node.owner)
		if err != nil {
			return nil, err
		}
		integer, err = t.integerSort(sort)
		if err != nil {
			return nil, err
		}
	}
	numeral, err := state.manager.ArithmeticNumeralRat(&value, integer)
	if err != nil {
		return nil, err
	}
	return NewSMTExpressionValueProcedure(numeral)
}

func (t *SMTRealDiffTheory) computeModelDelta() {
	t.delta.SetInt64(1)
	for index := range t.realGraph.edges {
		edge := &t.realGraph.edges[index]
		if !edge.enabled {
			continue
		}
		target := &t.realGraph.assignment[edge.target]
		source := &t.realGraph.assignment[edge.source]
		var rationalLimit, infinitesimalLimit big.Rat
		rationalLimit.Add(&source.rational, &edge.weight.rational)
		infinitesimalLimit.Add(&source.infinitesimal, &edge.weight.infinitesimal)
		if target.rational.Cmp(&rationalLimit) >= 0 || target.infinitesimal.Cmp(&infinitesimalLimit) <= 0 {
			continue
		}
		var numerator, denominator, candidate big.Rat
		numerator.Sub(&rationalLimit, &target.rational)
		denominator.Sub(&target.infinitesimal, &infinitesimalLimit)
		candidate.Quo(&numerator, &denominator)
		if candidate.Cmp(&t.delta) < 0 {
			t.delta.Set(&candidate)
		}
	}
}
