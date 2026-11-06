package smtcore

// Source: retained integer fragment of src/smt/theory_diff_logic.h and
// src/smt/theory_diff_logic_def.h.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
)

type smtIntegerDiffAtom struct {
	variable           int32
	positive, negative int32
	truth              bool
}

type smtIntegerDiffAssertion struct {
	atom  int
	truth bool
}

type smtIntegerDiffScope struct {
	atoms, assertions int
	head              int
	nonDifference     bool
}

type smtDiffStatistics struct {
	conflicts, assertions, coreEqualities, coreDisequalities uint32
}

// SMTIntegerDiffTheory is the concrete integer difference-logic theory. It
// accepts bounds of the form x-y <= k (and the equivalent >= form), where k
// is an integer numeral. Unsupported arithmetic is recorded for final check.
type SMTIntegerDiffTheory struct {
	*SMTTheoryBase
	graph           *SMTRationalDiffGraph
	zero            SMTTheoryVar
	atoms           []smtIntegerDiffAtom
	atomByBool      map[int32]int
	assertions      []smtIntegerDiffAssertion
	assertionHead   int
	scopes          []smtIntegerDiffScope
	nonDifference   bool
	addBinaryBounds bool
	equalityAdapter *smtArithmeticEqualityAdapter
	optimization    smtDiffOptimization
	statistics      smtDiffStatistics
}

func (t *SMTIntegerDiffTheory) ConfigureArithmetic(config SMTArithState) error {
	if t == nil || t.equalityAdapter == nil {
		return ast.ErrASTHandle
	}
	t.addBinaryBounds = config.AddBinaryBounds
	t.equalityAdapter.binary = config.AddBinaryBounds
	t.equalityAdapter.lazy = config.LazyAdapter
	return nil
}

func NewSMTIntegerDiffTheory(family ast.FamilyID) (*SMTIntegerDiffTheory, error) {
	base, err := NewSMTTheoryBase(family)
	if err != nil {
		return nil, err
	}
	theory := &SMTIntegerDiffTheory{
		SMTTheoryBase: base,
		graph:         NewSMTRationalDiffGraph(),
		zero:          SMTNullTheoryVar,
		atomByBool:    make(map[int32]int),
	}
	theory.equalityAdapter, err = newSMTArithmeticEqualityAdapter(base)
	if err != nil {
		return nil, err
	}
	return theory, nil
}

func (t *SMTIntegerDiffTheory) Initialize(state *SMTInternalizerState) error {
	if t == nil || t.SMTTheoryBase == nil || t.graph == nil {
		return ast.ErrASTHandle
	}
	if err := t.SMTTheoryBase.Initialize(state); err != nil {
		return err
	}
	zero, err := state.manager.ArithmeticNumeralRat(new(big.Rat), true)
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
	t.zero, err = t.createVariable(node)
	return err
}

func (t *SMTIntegerDiffTheory) createVariable(node *SMTEnode) (SMTTheoryVar, error) {
	if t.IsAttachedToVariable(node) {
		return node.TheoryVar(t.FamilyID()), nil
	}
	variable, err := t.SMTTheoryBase.CreateVariable(node)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if err = t.graph.InitVariable(int32(variable)); err != nil {
		return SMTNullTheoryVar, err
	}
	return variable, nil
}

func (t *SMTIntegerDiffTheory) integerSort(sort ast.ASTHandle) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	family, err := state.manager.GetFamilyID(sort)
	if err != nil || family != t.FamilyID() {
		return false, err
	}
	kind, err := state.manager.GetDeclKind(sort)
	return kind == 1, err
}

func (t *SMTIntegerDiffTheory) ApplySortConstraint(node *SMTEnode, sort ast.ASTHandle) error {
	integer, err := t.integerSort(sort)
	if err != nil {
		return err
	}
	if !integer {
		t.nonDifference = true
		return nil
	}
	_, err = t.createVariable(node)
	return err
}

func (t *SMTIntegerDiffTheory) numeral(expression ast.ASTHandle) (*big.Rat, bool, error) {
	state, err := t.State()
	if err != nil {
		return nil, false, err
	}
	text, _, matched, err := state.manager.ArithmeticNumeralText(expression)
	if err != nil || !matched {
		return nil, false, err
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, false, ast.ErrASTDomain
	}
	return value, true, nil
}

func (t *SMTIntegerDiffTheory) operation(expression ast.ASTHandle) (int32, bool, error) {
	state, err := t.State()
	if err != nil {
		return 0, false, err
	}
	kind, err := state.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return 0, false, err
	}
	family, err := state.manager.GetFamilyID(expression)
	if err != nil || family != t.FamilyID() {
		return 0, false, err
	}
	op, err := state.manager.GetDeclKind(expression)
	return op, err == nil, err
}

func (t *SMTIntegerDiffTheory) addFixedValue(variable SMTTheoryVar, value *big.Rat) error {
	forward, err := t.graph.AddEdge(int32(t.zero), int32(variable), value, SMTNullLiteral)
	if err != nil {
		return err
	}
	if feasible, enableErr := t.graph.EnableEdge(forward); enableErr != nil || !feasible {
		if enableErr != nil {
			return enableErr
		}
		return ast.ErrASTDomain
	}
	negative := new(big.Rat).Neg(value)
	backward, err := t.graph.AddEdge(int32(variable), int32(t.zero), negative, SMTNullLiteral)
	if err != nil {
		return err
	}
	if feasible, enableErr := t.graph.EnableEdge(backward); enableErr != nil || !feasible {
		if enableErr != nil {
			return enableErr
		}
		return ast.ErrASTDomain
	}
	return nil
}

func (t *SMTIntegerDiffTheory) offset(expression ast.ASTHandle) (ast.ASTHandle, *big.Rat, bool, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, nil, false, err
	}
	op, arithmetic, err := t.operation(expression)
	if err != nil || !arithmetic || op != smtArithOpAdd {
		return ast.ASTHandle{}, nil, false, err
	}
	count, err := state.manager.GetNumArgs(expression)
	if err != nil || count != 2 {
		return ast.ASTHandle{}, nil, false, err
	}
	left, err := state.manager.GetArg(expression, 0)
	if err != nil {
		return ast.ASTHandle{}, nil, false, err
	}
	right, err := state.manager.GetArg(expression, 1)
	if err != nil {
		return ast.ASTHandle{}, nil, false, err
	}
	if value, numeral, numeralErr := t.numeral(left); numeralErr != nil {
		return ast.ASTHandle{}, nil, false, numeralErr
	} else if numeral {
		return right, value, true, nil
	}
	if value, numeral, numeralErr := t.numeral(right); numeralErr != nil {
		return ast.ASTHandle{}, nil, false, numeralErr
	} else if numeral {
		return left, value, true, nil
	}
	return ast.ASTHandle{}, nil, false, nil
}

func (t *SMTIntegerDiffTheory) addOffset(source, target SMTTheoryVar, value *big.Rat) error {
	forward, err := t.graph.AddEdge(int32(source), int32(target), value, SMTNullLiteral)
	if err != nil {
		return err
	}
	if feasible, enableErr := t.graph.EnableEdge(forward); enableErr != nil || !feasible {
		if enableErr != nil {
			return enableErr
		}
		return ast.ErrASTDomain
	}
	negative := new(big.Rat).Neg(value)
	backward, err := t.graph.AddEdge(int32(target), int32(source), negative, SMTNullLiteral)
	if err != nil {
		return err
	}
	if feasible, enableErr := t.graph.EnableEdge(backward); enableErr != nil || !feasible {
		if enableErr != nil {
			return enableErr
		}
		return ast.ErrASTDomain
	}
	return nil
}

func (t *SMTIntegerDiffTheory) addBoundAxioms(source, target SMTTheoryVar, weight *big.Rat, literal SMTLiteral) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	equal, lower, upper := SMTNullLiteral, SMTNullLiteral, SMTNullLiteral
	var lowerWeight, upperWeight big.Rat
	for _, edgeID := range t.graph.outEdges[source] {
		edge := &t.graph.edges[edgeID]
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

func (t *SMTIntegerDiffTheory) InternalizeTerm(expression ast.ASTHandle) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	if node := state.context.enodes[expression]; node != nil {
		if !t.IsAttachedToVariable(node) {
			_, err = t.createVariable(node)
		}
		return err == nil, err
	}
	value, numeral, err := t.numeral(expression)
	if err != nil {
		return false, err
	}
	if numeral {
		if value.Sign() == 0 {
			return true, nil
		}
		node, mkErr := state.MkEnode(expression, false, false, true)
		if mkErr != nil {
			return false, mkErr
		}
		variable, mkErr := t.createVariable(node)
		if mkErr != nil {
			return false, mkErr
		}
		return true, t.addFixedValue(variable, value)
	}
	base, value, offset, err := t.offset(expression)
	if err != nil {
		return false, err
	}
	if offset {
		source, sourceErr := t.ensureVariable(base)
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
		target, mkErr := t.createVariable(node)
		if mkErr != nil {
			return false, mkErr
		}
		return true, t.addOffset(source, target, value)
	}
	_, arithmetic, err := t.operation(expression)
	if err != nil {
		return false, err
	}
	if arithmetic {
		t.nonDifference = true
		return false, nil
	}
	return false, nil
}

func (t *SMTIntegerDiffTheory) ensureVariable(expression ast.ASTHandle) (SMTTheoryVar, error) {
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
		// Arithmetic zero is represented by the theory's canonical zero node.
		if value, numeral, numeralErr := t.numeral(expression); numeralErr == nil && numeral && value.Sign() == 0 {
			return t.zero, nil
		}
		return SMTNullTheoryVar, ast.ErrASTDomain
	}
	if t.IsAttachedToVariable(node) {
		return node.TheoryVar(t.FamilyID()), nil
	}
	t.nonDifference = true
	return t.createVariable(node)
}

func (t *SMTIntegerDiffTheory) sign(expression ast.ASTHandle) (bool, bool, error) {
	if value, numeral, err := t.numeral(expression); err != nil {
		return false, false, err
	} else if numeral {
		if value.Cmp(big.NewRat(1, 1)) == 0 {
			return true, true, nil
		}
		if value.Cmp(big.NewRat(-1, 1)) == 0 {
			return false, true, nil
		}
	}
	op, arithmetic, err := t.operation(expression)
	if err != nil || !arithmetic || op != smtArithOpUMinus {
		return false, false, err
	}
	state, _ := t.State()
	argument, err := state.manager.GetArg(expression, 0)
	if err != nil {
		return false, false, err
	}
	sign, matched, err := t.sign(argument)
	return !sign, matched, err
}

func (t *SMTIntegerDiffTheory) decompose(expression ast.ASTHandle) ([]ast.ASTHandle, []bool, error) {
	state, err := t.State()
	if err != nil {
		return nil, nil, err
	}
	terms := []ast.ASTHandle{expression}
	signs := []bool{true}
	for index := 0; index < len(terms); index++ {
		op, arithmetic, opErr := t.operation(terms[index])
		if opErr != nil {
			return nil, nil, opErr
		}
		if !arithmetic {
			continue
		}
		switch op {
		case smtArithOpAdd:
			parent := terms[index]
			count, countErr := state.manager.GetNumArgs(parent)
			if countErr != nil || count == 0 {
				return nil, nil, ast.ErrASTDomain
			}
			first, argErr := state.manager.GetArg(parent, 0)
			if argErr != nil {
				return nil, nil, argErr
			}
			originalSign := signs[index]
			terms[index] = first
			for arg := uint32(1); arg < count; arg++ {
				child, childErr := state.manager.GetArg(parent, arg)
				if childErr != nil {
					return nil, nil, childErr
				}
				terms = append(terms, child)
				signs = append(signs, originalSign)
			}
			index--
		case smtArithOpMul:
			count, countErr := state.manager.GetNumArgs(terms[index])
			if countErr != nil {
				return nil, nil, countErr
			}
			if count != 2 {
				continue
			}
			left, leftErr := state.manager.GetArg(terms[index], 0)
			if leftErr != nil {
				return nil, nil, leftErr
			}
			right, rightErr := state.manager.GetArg(terms[index], 1)
			if rightErr != nil {
				return nil, nil, rightErr
			}
			coefficient, matched, signErr := t.sign(left)
			term := right
			if signErr != nil {
				return nil, nil, signErr
			}
			if !matched {
				coefficient, matched, signErr = t.sign(right)
				term = left
			}
			if signErr != nil {
				return nil, nil, signErr
			}
			if matched {
				terms[index] = term
				signs[index] = signs[index] == coefficient
				index--
			}
		case smtArithOpUMinus:
			child, childErr := state.manager.GetArg(terms[index], 0)
			if childErr != nil {
				return nil, nil, childErr
			}
			terms[index], signs[index] = child, !signs[index]
			index--
		}
	}
	return terms, signs, nil
}

func (t *SMTIntegerDiffTheory) InternalizeAtom(expression ast.ASTHandle, _ bool) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	op, arithmetic, err := t.operation(expression)
	if err != nil {
		return false, err
	}
	if !arithmetic || op != smtArithOpLE && op != smtArithOpGE {
		t.nonDifference = true
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
	if !numeral || !bound.IsInt() {
		t.nonDifference = true
		return false, nil
	}
	terms, signs, err := t.decompose(lhs)
	if err != nil {
		return false, err
	}
	var source, target SMTTheoryVar
	if len(terms) == 2 && signs[0] != signs[1] {
		target, err = t.ensureVariable(terms[0])
		if err == nil {
			source, err = t.ensureVariable(terms[1])
		}
		if err != nil {
			return false, err
		}
		if !signs[0] {
			target, source = source, target
		}
	} else {
		target, err = t.ensureVariable(lhs)
		if err != nil {
			return false, err
		}
		source = t.zero
	}
	weight := new(big.Rat).Set(bound)
	if isGE {
		target, source = source, target
		weight.Neg(weight)
	}
	variable, err := state.MkBoolVar(expression)
	if err != nil {
		return false, err
	}
	if err = state.SetVariableTheory(variable, t.FamilyID()); err != nil {
		return false, err
	}
	literal := NewSMTLiteral(variable, false)
	if err = t.addBoundAxioms(source, target, weight, literal); err != nil {
		return false, err
	}
	positive, err := t.graph.AddEdge(int32(source), int32(target), weight, literal)
	if err != nil {
		return false, err
	}
	negativeWeight := new(big.Rat).Neg(weight)
	negativeWeight.Sub(negativeWeight, big.NewRat(1, 1))
	negative, err := t.graph.AddEdge(int32(target), int32(source), negativeWeight, literal.Negated())
	if err != nil {
		return false, err
	}
	index := len(t.atoms)
	t.atoms = append(t.atoms, smtIntegerDiffAtom{variable: variable, positive: positive, negative: negative})
	t.atomByBool[variable] = index
	return true, nil
}

func (t *SMTIntegerDiffTheory) Assign(variable int32, truth bool) error {
	index, found := t.atomByBool[variable]
	if !found {
		return ast.ErrASTDomain
	}
	t.atoms[index].truth = truth
	t.assertions = append(t.assertions, smtIntegerDiffAssertion{atom: index, truth: truth})
	t.statistics.assertions++
	return nil
}

func (t *SMTIntegerDiffTheory) CanPropagate() bool {
	return t != nil && t.assertionHead < len(t.assertions)
}

func (t *SMTIntegerDiffTheory) Propagate() error {
	state, err := t.State()
	if err != nil {
		return err
	}
	for !state.inconsistent && t.CanPropagate() {
		assertion := t.assertions[t.assertionHead]
		t.assertionHead++
		atom := &t.atoms[assertion.atom]
		edge := atom.negative
		if assertion.truth {
			edge = atom.positive
		}
		feasible, enableErr := t.graph.EnableEdge(edge)
		if enableErr != nil {
			return enableErr
		}
		if !feasible {
			literals, explainErr := t.graph.NegativeCycleExplanations(true)
			if explainErr != nil {
				return explainErr
			}
			kept := literals[:0]
			for _, literal := range literals {
				if literal != SMTNullLiteral {
					kept = append(kept, literal)
				}
			}
			if err = state.SetTheoryConflict(t.FamilyID(), kept); err == nil {
				t.statistics.conflicts++
			}
			return err
		}
	}
	return nil
}

func (t *SMTIntegerDiffTheory) PushScope() error {
	if t == nil || t.SMTTheoryBase == nil || t.graph == nil {
		return ast.ErrASTHandle
	}
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	if err := t.graph.Push(); err != nil {
		_ = t.SMTTheoryBase.PopScopes(1)
		return err
	}
	if err := t.equalityAdapter.PushScope(); err != nil {
		_ = t.graph.Pop(1)
		_ = t.SMTTheoryBase.PopScopes(1)
		return err
	}
	t.scopes = append(t.scopes, smtIntegerDiffScope{len(t.atoms), len(t.assertions), t.assertionHead, t.nonDifference})
	return nil
}

func (t *SMTIntegerDiffTheory) PopScopes(count uint32) error {
	if t == nil || count == 0 || int(count) > len(t.scopes) {
		return ast.ErrASTDomain
	}
	level := len(t.scopes) - int(count)
	scope := t.scopes[level]
	for index := len(t.atoms); index > scope.atoms; index-- {
		delete(t.atomByBool, t.atoms[index-1].variable)
	}
	t.atoms = t.atoms[:scope.atoms]
	t.assertions = t.assertions[:scope.assertions]
	t.assertionHead = scope.head
	t.nonDifference = scope.nonDifference
	t.scopes = t.scopes[:level]
	if err := t.equalityAdapter.PopScopes(count); err != nil {
		return err
	}
	if err := t.graph.Pop(count); err != nil {
		return err
	}
	return t.SMTTheoryBase.PopScopes(count)
}

func (t *SMTIntegerDiffTheory) FinalCheck() (SMTFinalCheckStatus, error) {
	if t.CanPropagate() {
		if err := t.Propagate(); err != nil {
			return SMTFinalCheckGiveUp, err
		}
		return SMTFinalCheckContinue, nil
	}
	if err := t.graph.SetToZero(int32(t.zero)); err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if t.nonDifference {
		return SMTFinalCheckGiveUp, nil
	}
	return SMTFinalCheckDone, nil
}

func (t *SMTIntegerDiffTheory) InitializeSearch() error {
	return t.equalityAdapter.InitializeSearch()
}

func (t *SMTIntegerDiffTheory) Restart() error {
	return t.equalityAdapter.Restart()
}

func (t *SMTIntegerDiffTheory) InternalizeEquality(expression ast.ASTHandle, variable int32) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	left, err := state.manager.GetArg(expression, 0)
	if err != nil {
		return err
	}
	right, err := state.manager.GetArg(expression, 1)
	if err != nil {
		return err
	}
	minusOne, err := state.manager.ArithmeticNumeralRat(big.NewRat(-1, 1), true)
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
	zero, err := state.manager.ArithmeticNumeralRat(new(big.Rat), true)
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
func (t *SMTIntegerDiffTheory) NewEquality(left, right SMTTheoryVar) error {
	return t.equalityAdapter.NewEquality(left, right)
}
func (t *SMTIntegerDiffTheory) NewDisequality(left, right SMTTheoryVar) error {
	return t.equalityAdapter.NewDisequality(left, right)
}

func (t *SMTIntegerDiffTheory) Graph() *SMTRationalDiffGraph {
	if t == nil {
		return nil
	}
	return t.graph
}

func (*SMTIntegerDiffTheory) Name() string { return "difference-logic" }

func (t *SMTIntegerDiffTheory) InitializeModel(generator *SMTModelGenerator) error {
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
	factory, err := protomodel.NewArithmeticFactory(state.manager)
	if err != nil {
		return err
	}
	return generator.RegisterFactory(factory)
}

func (t *SMTIntegerDiffTheory) ModelValue(node *SMTEnode, generator *SMTModelGenerator) (SMTModelValueProcedure, error) {
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
	value, err := t.graph.Assignment(int32(variable))
	if err != nil {
		return nil, err
	}
	numeral, err := state.manager.ArithmeticNumeralRat(value, true)
	if err != nil {
		return nil, err
	}
	return NewSMTExpressionValueProcedure(numeral)
}
