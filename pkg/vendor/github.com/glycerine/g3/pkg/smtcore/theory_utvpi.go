package smtcore

// Source: retained rdl_ext instantiation of src/smt/theory_utvpi.h/.def.h.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type smtUTVPIScope struct {
	atoms, assertions, head int
	unsupported             bool
}

type smtUTVPIStatistics struct {
	conflicts, assertions, coreEqualities uint32
}

// SMTUTVPITheory owns the source doubled-variable graph. Its arithmetic
// equality adapter has constructor defaults, as in the source plugin, rather
// than borrowing the context's arithmetic parameter object.
type SMTUTVPITheory struct {
	*SMTTheoryBase
	syntax      SMTIntegerDiffTheory
	adapter     *smtArithmeticEqualityAdapter
	linearizer  *smtUTVPILinearizer
	graph       *smtUTVPIGraph
	zero        SMTTheoryVar
	atoms       []smtIntegerDiffAtom
	byBool      map[int32]int
	assertions  []smtIntegerDiffAssertion
	head        int
	scopes      []smtUTVPIScope
	unsupported bool
	values      []big.Rat
	statistics  smtUTVPIStatistics
}

func NewSMTUTVPITheory(family ast.FamilyID) (*SMTUTVPITheory, error) {
	base, err := NewSMTTheoryBase(family)
	if err != nil {
		return nil, err
	}
	adapter, err := newSMTArithmeticEqualityAdapter(base)
	if err != nil {
		return nil, err
	}
	t := &SMTUTVPITheory{SMTTheoryBase: base, adapter: adapter, graph: newSMTUTVPIGraph(), zero: SMTNullTheoryVar, byBool: make(map[int32]int)}
	t.syntax.SMTTheoryBase, t.syntax.equalityAdapter = base, adapter
	return t, nil
}

func (*SMTUTVPITheory) Name() string { return "utvpi-logic" }

func (t *SMTUTVPITheory) Initialize(state *SMTInternalizerState) error {
	if err := t.SMTTheoryBase.Initialize(state); err != nil {
		return err
	}
	var err error
	t.linearizer, err = newSMTUTVPILinearizer(state.manager)
	if err != nil {
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

func (t *SMTUTVPITheory) createVariable(node *SMTEnode) (SMTTheoryVar, error) {
	if t.IsAttachedToVariable(node) {
		return node.TheoryVar(t.FamilyID()), nil
	}
	sort, err := t.state.manager.GetSort(node.owner)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	integer, err := t.syntax.integerSort(sort)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	v, err := t.SMTTheoryBase.CreateVariable(node)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	graphVar, err := t.graph.addVariable(integer)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if graphVar != int32(v) {
		return SMTNullTheoryVar, ast.ErrASTDomain
	}
	return v, nil
}

func (t *SMTUTVPITheory) variable(expression ast.ASTHandle) (SMTTheoryVar, error) {
	if node := t.state.context.enodes[expression]; node != nil && t.IsAttachedToVariable(node) {
		return node.TheoryVar(t.FamilyID()), nil
	}
	if err := t.state.Internalize(expression, false); err != nil {
		return SMTNullTheoryVar, err
	}
	node := t.state.context.enodes[expression]
	if node == nil {
		return SMTNullTheoryVar, ast.ErrASTDomain
	}
	return t.createVariable(node)
}

func (t *SMTUTVPITheory) ApplySortConstraint(node *SMTEnode, _ ast.ASTHandle) error {
	family, err := t.state.manager.GetFamilyID(node.owner)
	if err != nil {
		return err
	}
	if family == t.FamilyID() && !t.IsAttachedToVariable(node) {
		// Unsupported arithmetic terms can still have enodes for congruence
		// closure, but cannot silently become unconstrained graph variables.
		t.unsupported = true
	}
	_, err = t.createVariable(node)
	return err
}

func (t *SMTUTVPITheory) InternalizeAtom(expression ast.ASTHandle, _ bool) (bool, error) {
	op, own, err := t.syntax.operation(expression)
	if err != nil {
		return false, err
	}
	if !own || op != smtArithOpLE && op != smtArithOpGE && op != smtArithOpLT && op != smtArithOpGT {
		t.unsupported = true
		return false, nil
	}
	m := t.state.manager
	left, err := m.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	right, err := m.GetArg(expression, 1)
	if err != nil {
		return false, err
	}
	if op == smtArithOpGE || op == smtArithOpGT {
		left, right = right, left
	}
	accepted, err := t.linearizer.linearize(left, right)
	if err != nil || !accepted {
		t.unsupported = true
		return false, err
	}
	var weight big.Rat
	weight.Set(&t.linearizer.weight)
	terms := append([]smtUTVPITerm(nil), t.linearizer.terms...)
	coefficients := make([]smtUTVPIGraphTerm, len(terms))
	for i, term := range terms {
		variable, err := t.variable(term.expression)
		if err != nil {
			return false, err
		}
		coefficients[i] = smtUTVPIGraphTerm{int32(variable), term.coefficient.Sign() < 0}
	}
	sort, err := m.GetSort(left)
	if err != nil {
		return false, err
	}
	integer, err := t.syntax.integerSort(sort)
	if err != nil {
		return false, err
	}
	strict := op == smtArithOpLT || op == smtArithOpGT
	positiveWeight := newSMTInfRational(&weight, nil)
	negativeWeight := newSMTInfRational(new(big.Rat).Neg(&weight), nil)
	step := &negativeWeight
	if strict {
		step = &positiveWeight
	}
	if integer {
		step.rational.Add(&step.rational, big.NewRat(1, 1))
	} else {
		step.infinitesimal.SetInt64(1)
	}
	v, err := t.state.MkBoolVar(expression)
	if err != nil {
		return false, err
	}
	if err := t.state.SetVariableTheory(v, t.FamilyID()); err != nil {
		return false, err
	}
	literal := NewSMTLiteral(v, false)
	if len(coefficients) == 0 {
		// Rewriting usually folds this case before theory internalization.
		if positiveWeight.Sign() > 0 {
			literal = literal.Negated()
		}
		return true, t.state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{literal})
	}
	positive, err := t.graph.addInequality(coefficients, &positiveWeight, literal)
	if err != nil {
		return false, err
	}
	for i := range coefficients {
		coefficients[i].negative = !coefficients[i].negative
	}
	negative, err := t.graph.addInequality(coefficients, &negativeWeight, literal.Negated())
	if err != nil {
		return false, err
	}
	t.byBool[v] = len(t.atoms)
	t.atoms = append(t.atoms, smtIntegerDiffAtom{variable: v, positive: positive, negative: negative})
	return true, nil
}

func (t *SMTUTVPITheory) InternalizeTerm(expression ast.ASTHandle) (bool, error) {
	if node := t.state.context.enodes[expression]; node != nil && t.IsAttachedToVariable(node) {
		return true, nil
	}
	accepted, err := t.linearizer.linearize(expression, ast.ASTHandle{})
	if err != nil || !accepted {
		t.unsupported = true
		return false, err
	}
	if len(t.linearizer.terms) == 2 {
		// As in mk_term, a two-variable sum is an atom operand, not an alias.
		return false, nil
	}
	var weight big.Rat
	weight.Set(&t.linearizer.weight)
	terms := append([]smtUTVPITerm(nil), t.linearizer.terms...)
	graphTerms := []smtUTVPIGraphTerm{}
	for _, term := range terms {
		v, err := t.variable(term.expression)
		if err != nil {
			return false, err
		}
		graphTerms = append(graphTerms, smtUTVPIGraphTerm{int32(v), term.coefficient.Sign() < 0})
	}
	node := t.state.context.enodes[expression]
	if node == nil {
		node, err = t.state.MkEnode(expression, false, false, true)
		if err != nil {
			return false, err
		}
	}
	v, err := t.createVariable(node)
	if err != nil {
		return false, err
	}
	graphTerms = append(graphTerms, smtUTVPIGraphTerm{int32(v), true})
	for i := 0; i < 2; i++ {
		w := newSMTInfRational(&weight, nil)
		id, err := t.graph.addInequality(graphTerms, &w, SMTNullLiteral)
		if err != nil {
			return false, err
		}
		ok, err := t.graph.enablePair(id)
		if err != nil || !ok {
			return false, ast.ErrASTDomain
		}
		for j := range graphTerms {
			graphTerms[j].negative = !graphTerms[j].negative
		}
		weight.Neg(&weight)
	}
	return true, nil
}

func (t *SMTUTVPITheory) Assign(variable int32, truth bool) error {
	if i, found := t.byBool[variable]; found {
		t.statistics.assertions++
		t.assertions = append(t.assertions, smtIntegerDiffAssertion{atom: i, truth: truth})
	}
	return nil
}

func (t *SMTUTVPITheory) CanPropagate() bool { return t.head < len(t.assertions) }

func (t *SMTUTVPITheory) setConflict(edges []int32) (result error) {
	t.statistics.conflicts++
	var antecedents smtArithmeticAntecedents
	for _, id := range edges {
		edge := &t.graph.graph.edges[id]
		if edge.explanation == SMTNullLiteral {
			continue
		}
		antecedents.literals = append(antecedents.literals, edge.explanation)
		antecedents.literalCoefficients = append(antecedents.literalCoefficients, big.NewRat(int64(t.graph.coefficients[id]), 1))
	}
	var parameters []ast.ASTParameter
	proofs, err := t.state.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	if proofs {
		parameters, err = antecedents.parameters(t.state.manager, "farkas")
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, smtArithmeticReleaseParameters(t.state.manager, parameters)) }()
	}
	owner, err := NewSMTExtTheoryConflictJustification(t.state.manager, t.FamilyID(), antecedents.literals, nil, parameters)
	if err != nil {
		return err
	}
	wrapped, err := NewSMTBooleanJustification(owner)
	if err != nil {
		return errors.Join(err, owner.Release(t.state.manager))
	}
	t.state.ownedJustifications = append(t.state.ownedJustifications, owner)
	t.state.inconsistent = true
	t.state.conflict = wrapped
	t.state.conflictLiteral = SMTNullLiteral
	t.state.statistics.NumConflicts++
	return nil
}

func (t *SMTUTVPITheory) Propagate() error {
	for !t.state.inconsistent && t.CanPropagate() {
		assertion := t.assertions[t.head]
		t.head++
		atom := &t.atoms[assertion.atom]
		edge := atom.negative
		if assertion.truth {
			edge = atom.positive
		}
		ok, err := t.graph.enablePair(edge)
		if err != nil {
			return err
		}
		if !ok {
			cycle, err := t.graph.negativeCycleEdges()
			if err != nil {
				return err
			}
			return t.setConflict(cycle)
		}
	}
	return nil
}

func (t *SMTUTVPITheory) FinalCheck() (SMTFinalCheckStatus, error) {
	if t.CanPropagate() {
		return SMTFinalCheckContinue, t.Propagate()
	}
	conflict, err := t.graph.parityConflict()
	if err != nil {
		return SMTFinalCheckGiveUp, nil
	}
	if len(conflict) > 0 {
		return SMTFinalCheckContinue, t.setConflict(conflict)
	}
	if t.unsupported {
		return SMTFinalCheckGiveUp, nil
	}
	return SMTFinalCheckDone, nil
}

func (t *SMTUTVPITheory) InternalizeEquality(expression ast.ASTHandle, variable int32) error {
	left, err := t.state.manager.GetArg(expression, 0)
	if err != nil {
		return err
	}
	sort, err := t.state.manager.GetSort(left)
	if err != nil {
		return err
	}
	integer, err := t.syntax.integerSort(sort)
	if err != nil {
		return err
	}
	if integer {
		return t.syntax.InternalizeEquality(expression, variable)
	}
	real := SMTRealDiffTheory{SMTIntegerDiffTheory: &t.syntax}
	return real.InternalizeEquality(expression, variable)
}

func (t *SMTUTVPITheory) NewEquality(left, right SMTTheoryVar) error {
	t.statistics.coreEqualities++
	return t.adapter.NewEquality(left, right)
}
func (t *SMTUTVPITheory) NewDisequality(left, right SMTTheoryVar) error {
	return t.adapter.NewDisequality(left, right)
}
func (t *SMTUTVPITheory) InitializeSearch() error { return t.adapter.InitializeSearch() }
func (t *SMTUTVPITheory) Restart() error          { return t.adapter.Restart() }

func (t *SMTUTVPITheory) PushScope() error {
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	if err := t.graph.push(); err != nil {
		return err
	}
	if err := t.adapter.PushScope(); err != nil {
		return err
	}
	t.scopes = append(t.scopes, smtUTVPIScope{len(t.atoms), len(t.assertions), t.head, t.unsupported})
	return nil
}

func (t *SMTUTVPITheory) PopScopes(count uint32) error {
	if count == 0 || int(count) > len(t.scopes) {
		return ast.ErrASTDomain
	}
	level := len(t.scopes) - int(count)
	scope := t.scopes[level]
	for _, atom := range t.atoms[scope.atoms:] {
		delete(t.byBool, atom.variable)
	}
	t.atoms, t.assertions = t.atoms[:scope.atoms], t.assertions[:scope.assertions]
	t.head, t.unsupported = scope.head, scope.unsupported
	t.scopes, t.values = t.scopes[:level], nil
	if err := t.adapter.PopScopes(count); err != nil {
		return err
	}
	if err := t.graph.pop(count); err != nil {
		return err
	}
	return t.SMTTheoryBase.PopScopes(count)
}

func (t *SMTUTVPITheory) InitializeModel(generator *SMTModelGenerator) error {
	if generator == nil || generator.manager != t.state.manager {
		return ast.ErrASTDomain
	}
	var err error
	t.values, err = t.graph.modelValues(int32(t.zero))
	if err != nil {
		return err
	}
	factory, err := protomodel.NewArithmeticFactory(t.state.manager)
	if err != nil {
		return err
	}
	return generator.RegisterFactory(factory)
}

func (t *SMTUTVPITheory) ModelValue(node *SMTEnode, generator *SMTModelGenerator) (SMTModelValueProcedure, error) {
	if node == nil || generator == nil || generator.manager != t.state.manager {
		return nil, ast.ErrASTDomain
	}
	v := node.TheoryVar(t.FamilyID())
	if !t.IsAttachedToVariable(node) || v < 0 || int(v) >= len(t.values) {
		return nil, ast.ErrASTDomain
	}
	expression, err := t.state.manager.ArithmeticNumeralRat(&t.values[v], t.graph.integer[v])
	if err != nil {
		return nil, err
	}
	return NewSMTExpressionValueProcedure(expression)
}

func (t *SMTUTVPITheory) CollectStatistics(target *statistics.Statistics) error {
	if t == nil || target == nil {
		return ast.ErrASTHandle
	}
	target.UpdateUint("utvpi conflicts", t.statistics.conflicts)
	target.UpdateUint("utvpi asserts", t.statistics.assertions)
	target.UpdateUint("core->utvpi eqs", t.statistics.coreEqualities)
	// The source core->utvpi diseqs counter is only incremented by the unused
	// three-argument overload. Zero contributions are omitted by statistics.
	target.UpdateUint64("arith eq adapter", t.adapter.NumAxioms())
	target.UpdateUint("dl prop steps", t.graph.graph.propagationCost)
	// UTVPI never calls the graph's implied-literal or relaxed-edge routines;
	// their four additional graph counters remain zero in this instantiation.
	return nil
}
