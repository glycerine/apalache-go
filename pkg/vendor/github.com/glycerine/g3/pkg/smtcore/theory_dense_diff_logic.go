package smtcore

// Source: src/smt/theory_dense_diff_logic.h and theory_dense_diff_logic_def.h.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type smtDenseAtom struct {
	variable       int32
	source, target int
	offset         smtInfRational
}
type smtDenseScope struct {
	atoms       int
	unsupported bool
}

// SMTDenseDiffTheory is the concrete arbitrary-precision dense arithmetic
// specialization. Syntax helpers borrow its base; no sparse graph is created.
type SMTDenseDiffTheory struct {
	*SMTTheoryBase
	syntax      SMTIntegerDiffTheory
	matrix      *smtDenseDiffMatrix
	adapter     *smtArithmeticEqualityAdapter
	config      SMTArithState
	atoms       []smtDenseAtom
	byBool      map[int32]int
	occurrences map[[2]int][]int
	integer     []bool
	scopes      []smtDenseScope
	unsupported bool
	values      []smtInfRational
	delta       big.Rat
	statistics  smtDenseDiffStatistics
}

func NewSMTDenseDiffTheory(family ast.FamilyID) (*SMTDenseDiffTheory, error) {
	base, err := NewSMTTheoryBase(family)
	if err != nil {
		return nil, err
	}
	adapter, err := newSMTArithmeticEqualityAdapter(base)
	if err != nil {
		return nil, err
	}
	t := &SMTDenseDiffTheory{SMTTheoryBase: base, matrix: newSMTDenseDiffMatrix(), adapter: adapter, byBool: make(map[int32]int), occurrences: make(map[[2]int][]int)}
	t.syntax.SMTTheoryBase, t.syntax.equalityAdapter = base, adapter
	t.matrix.owner = t
	return t, nil
}

func (t *SMTDenseDiffTheory) ConfigureArithmetic(config SMTArithState) error {
	t.config = config
	t.adapter.lazy = config.LazyAdapter
	t.adapter.binary = config.AddBinaryBounds
	return nil
}

func (*SMTDenseDiffTheory) Name() string { return "dense-difference-logic" }

func (t *SMTDenseDiffTheory) createVariable(node *SMTEnode) (SMTTheoryVar, error) {
	if t.IsAttachedToVariable(node) {
		return node.TheoryVar(t.FamilyID()), nil
	}
	v, err := t.SMTTheoryBase.CreateVariable(node)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if int(v) != t.matrix.addVariable() {
		return SMTNullTheoryVar, ast.ErrASTDomain
	}
	sort, err := t.state.manager.GetSort(node.owner)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	integer, err := t.syntax.integerSort(sort)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	t.integer = append(t.integer, integer)
	return v, nil
}

func (t *SMTDenseDiffTheory) term(expression ast.ASTHandle) (SMTTheoryVar, error) {
	m := t.state.manager
	if node := t.state.context.enodes[expression]; node != nil && t.IsAttachedToVariable(node) {
		return node.TheoryVar(t.FamilyID()), nil
	}
	value, numeral, err := t.syntax.numeral(expression)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	base, offset, shifted, err := t.syntax.offset(expression)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	var source SMTTheoryVar
	if shifted {
		if t.config.Reflect {
			for i := uint32(0); i < 2; i++ {
				argument, err := m.GetArg(expression, i)
				if err != nil {
					return SMTNullTheoryVar, err
				}
				if _, numeral, err := t.syntax.numeral(argument); err != nil {
					return SMTNullTheoryVar, err
				} else if numeral {
					if _, err := t.term(argument); err != nil {
						return SMTNullTheoryVar, err
					}
				}
			}
		}
		source, err = t.term(base)
		if err != nil || source == SMTNullTheoryVar {
			return source, err
		}
	} else if !numeral {
		_, arithmetic, err := t.syntax.operation(expression)
		if err != nil {
			return SMTNullTheoryVar, err
		}
		if arithmetic {
			return SMTNullTheoryVar, nil
		}
		if err := t.state.Internalize(expression, false); err != nil {
			return SMTNullTheoryVar, err
		}
	}
	node := t.state.context.enodes[expression]
	if node == nil {
		node, err = t.state.MkEnode(expression, shifted && !t.config.Reflect, false, true)
		if err != nil {
			return SMTNullTheoryVar, err
		}
	}
	v, err := t.createVariable(node)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if numeral && value.Sign() != 0 {
		zero, err := m.ArithmeticNumeralRat(new(big.Rat), t.integer[v])
		if err != nil {
			return SMTNullTheoryVar, err
		}
		source, err = t.term(zero)
		if err != nil {
			return SMTNullTheoryVar, err
		}
		offset, shifted = value, true
	}
	if shifted {
		weight := newSMTInfRational(offset, nil)
		if err = t.addEdge(int(source), int(v), &weight, SMTNullLiteral); err != nil {
			return SMTNullTheoryVar, err
		}
		weight.Neg(&weight)
		if err = t.addEdge(int(v), int(source), &weight, SMTNullLiteral); err != nil {
			return SMTNullTheoryVar, err
		}
	}
	return v, nil
}

func (t *SMTDenseDiffTheory) InternalizeTerm(expression ast.ASTHandle) (bool, error) {
	v, err := t.term(expression)
	if v == SMTNullTheoryVar {
		t.unsupported = true
	}
	return v != SMTNullTheoryVar, err
}

func (*SMTDenseDiffTheory) ApplySortConstraint(*SMTEnode, ast.ASTHandle) error { return nil }

func (t *SMTDenseDiffTheory) InternalizeAtom(expression ast.ASTHandle, _ bool) (bool, error) {
	op, own, err := t.syntax.operation(expression)
	if err != nil {
		return false, err
	}
	if !own || op != smtArithOpLE && op != smtArithOpGE {
		t.unsupported = true
		return false, nil
	}
	m := t.state.manager
	lhs, err := m.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	rhs, err := m.GetArg(expression, 1)
	if err != nil {
		return false, err
	}
	bound, numeral, err := t.syntax.numeral(rhs)
	if err != nil {
		return false, err
	}
	if !numeral {
		t.unsupported = true
		return false, nil
	}
	terms, signs, err := t.syntax.decompose(lhs)
	if err != nil {
		return false, err
	}
	var source, target ast.ASTHandle
	if len(terms) == 2 && signs[0] != signs[1] {
		target, source = terms[0], terms[1]
		if !signs[0] {
			source, target = target, source
		}
	} else if len(terms) == 1 {
		sort, err := m.GetSort(terms[0])
		if err != nil {
			return false, err
		}
		integer, err := t.syntax.integerSort(sort)
		if err != nil {
			return false, err
		}
		zero, err := m.ArithmeticNumeralRat(new(big.Rat), integer)
		if err != nil {
			return false, err
		}
		source, target = zero, terms[0]
		if !signs[0] {
			source, target = target, source
		}
	} else {
		t.unsupported = true
		return false, nil
	}
	s, err := t.term(source)
	if err != nil {
		return false, err
	}
	u, err := t.term(target)
	if err != nil {
		return false, err
	}
	if s == SMTNullTheoryVar || u == SMTNullTheoryVar {
		t.unsupported = true
		return false, nil
	}
	weight := newSMTInfRational(bound, nil)
	if op == smtArithOpGE {
		s, u = u, s
		weight.Neg(&weight)
	}
	v, err := t.state.MkBoolVar(expression)
	if err != nil {
		return false, err
	}
	if err = t.state.SetVariableTheory(v, t.FamilyID()); err != nil {
		return false, err
	}
	a := smtDenseAtom{v, int(s), int(u), weight}
	index := len(t.atoms)
	t.byBool[v] = index
	t.atoms = append(t.atoms, a)
	t.occurrences[[2]int{int(s), int(u)}] = append(t.occurrences[[2]int{int(s), int(u)}], index)
	t.occurrences[[2]int{int(u), int(s)}] = append(t.occurrences[[2]int{int(u), int(s)}], index)
	return true, nil
}

func (t *SMTDenseDiffTheory) Assign(variable int32, truth bool) error {
	data, err := t.state.BooleanData(variable)
	if err != nil {
		return err
	}
	if justification, ok := data.Justification().Justification(); ok && justification != nil && justification.FromTheory() == t.FamilyID() {
		return nil
	}
	i, found := t.byBool[variable]
	if !found {
		return nil
	}
	a := &t.atoms[i]
	t.statistics.assertions++
	source, target := a.source, a.target
	var offset smtInfRational
	offset.Set(&a.offset)
	if !truth {
		source, target = target, source
		offset.Neg(&offset)
		if t.integer[a.source] {
			offset.rational.Sub(&offset.rational, big.NewRat(1, 1))
		} else {
			offset.infinitesimal.Sub(&offset.infinitesimal, big.NewRat(1, 1))
		}
	}
	return t.addEdge(source, target, &offset, NewSMTLiteral(variable, !truth))
}

func (t *SMTDenseDiffTheory) addEdge(source, target int, offset *smtInfRational, literal SMTLiteral) error {
	ok, reason, err := t.matrix.addEdge(source, target, offset, literal)
	if err != nil {
		return err
	}
	if !ok {
		if err = t.state.SetTheoryConflict(t.FamilyID(), reason); err != nil {
			return err
		}
		if !t.config.DumpLemmas {
			return nil
		}
		_, err = t.state.DumpLemmaAsSMTProblem(reason, nil, SMTFalseLiteral, usymbol.Symbol{})
		var dump *SMTLemmaDumpError
		if errors.As(err, &dump) {
			return dump.OperandError
		}
		return err
	}
	return nil
}

func (t *SMTDenseDiffTheory) propagateCell(source, target int) error {
	distance, connected := t.matrix.distance(source, target)
	if !connected {
		return nil
	}
	for _, index := range t.occurrences[[2]int{source, target}] {
		a := &t.atoms[index]
		literal := NewSMTLiteral(a.variable, false)
		assignment, err := t.state.Assignment(literal)
		if err != nil {
			return err
		}
		if assignment != lbool.Undef {
			continue
		}
		implied := distance.Cmp(&a.offset) <= 0
		if a.source != source {
			var negative smtInfRational
			negative.Neg(&distance)
			implied = negative.Cmp(&a.offset) > 0
			literal = literal.Negated()
		}
		if !implied {
			continue
		}
		t.statistics.propagations++
		reason, err := t.matrix.antecedents(source, target)
		if err != nil {
			return err
		}
		owner, err := NewSMTTheoryPropagationJustification(t.state.manager, t.FamilyID(), reason, literal, nil)
		if err != nil {
			return err
		}
		justification, err := NewSMTBooleanJustification(owner)
		if err != nil {
			return errors.Join(err, owner.Release(t.state.manager))
		}
		if err = t.state.AssignLiteral(literal, justification); err != nil {
			return errors.Join(err, owner.Release(t.state.manager))
		}
		t.state.ownedJustifications = append(t.state.ownedJustifications, owner)
	}
	return nil
}

func (t *SMTDenseDiffTheory) InitializeSearch() error { return t.adapter.InitializeSearch() }

func (t *SMTDenseDiffTheory) PushScope() error {
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	if err := t.adapter.PushScope(); err != nil {
		return err
	}
	t.matrix.push()
	t.scopes = append(t.scopes, smtDenseScope{len(t.atoms), t.unsupported})
	return nil
}
func (t *SMTDenseDiffTheory) PopScopes(count uint32) error {
	if count == 0 || uint64(count) > uint64(len(t.scopes)) {
		return ast.ErrASTDomain
	}
	level := len(t.scopes) - int(count)
	s := t.scopes[level]
	for i := len(t.atoms) - 1; i >= s.atoms; i-- {
		a := &t.atoms[i]
		delete(t.byBool, a.variable)
		for _, key := range [][2]int{{a.source, a.target}, {a.target, a.source}} {
			list := t.occurrences[key]
			list = list[:len(list)-1]
			if len(list) == 0 {
				delete(t.occurrences, key)
			} else {
				t.occurrences[key] = list
			}
		}
	}
	t.atoms, t.unsupported = t.atoms[:s.atoms], s.unsupported
	t.scopes = t.scopes[:level]
	if err := t.matrix.pop(count); err != nil {
		return err
	}
	t.integer = t.integer[:len(t.matrix.rows)]
	if err := t.adapter.PopScopes(count); err != nil {
		return err
	}
	return t.SMTTheoryBase.PopScopes(count)
}

func (t *SMTDenseDiffTheory) FinalCheck() (SMTFinalCheckStatus, error) {
	t.values = t.matrix.assignment()
	values := make(map[smtArithmeticSharedValueKey]*SMTEnode)
	for v, node := range t.variables {
		if !t.state.isRelevant(node.owner) {
			continue
		}
		shared, err := smtSharedArithmeticNode(t.SMTTheoryBase, node, false)
		if err != nil {
			return SMTFinalCheckGiveUp, err
		}
		if !shared {
			continue
		}
		sort, err := t.state.manager.GetSort(node.owner)
		if err != nil {
			return SMTFinalCheckGiveUp, err
		}
		key := smtArithmeticSharedValueKey{sort, "0", t.values[v].rational.RatString(), t.values[v].infinitesimal.RatString()}
		if other := values[key]; other != nil && other.root != node.root {
			progress, err := t.state.assumeEquality(other, node)
			if err != nil {
				return SMTFinalCheckGiveUp, err
			}
			if progress {
				return SMTFinalCheckContinue, nil
			}
		} else {
			values[key] = node
		}
	}
	if t.unsupported {
		return SMTFinalCheckGiveUp, nil
	}
	return SMTFinalCheckDone, nil
}

func (t *SMTDenseDiffTheory) InitializeModel(generator *SMTModelGenerator) error {
	if generator == nil || generator.manager != t.state.manager || generator.model == nil {
		return ast.ErrASTDomain
	}
	if len(t.values) != t.NumVariables() {
		return ast.ErrASTDomain
	}
	factory, err := protomodel.NewArithmeticFactory(t.state.manager)
	if err != nil {
		return err
	}
	if err = generator.RegisterFactory(factory); err != nil {
		return err
	}
	for v, node := range t.variables {
		if value, numeral, err := t.syntax.numeral(node.owner); err != nil {
			return err
		} else if numeral && value.Sign() == 0 {
			var shift smtInfRational
			shift.Set(&t.values[v])
			sort, err := t.state.manager.GetSort(node.owner)
			if err != nil {
				return err
			}
			for other, n := range t.variables {
				otherSort, err := t.state.manager.GetSort(n.owner)
				if err != nil {
					return err
				}
				if otherSort == sort {
					t.values[other].Sub(&t.values[other], &shift)
				}
			}
		}
	}
	t.delta.SetInt64(1)
	for _, e := range t.matrix.edges[1:] {
		x, y := &t.values[e.target], &t.values[e.source]
		var limit, infinitesimal big.Rat
		limit.Add(&y.rational, &e.offset.rational)
		infinitesimal.Add(&y.infinitesimal, &e.offset.infinitesimal)
		if x.rational.Cmp(&limit) < 0 && x.infinitesimal.Cmp(&infinitesimal) > 0 {
			var numerator, denominator, candidate big.Rat
			numerator.Sub(&limit, &x.rational)
			denominator.Sub(&x.infinitesimal, &infinitesimal)
			candidate.Quo(&numerator, &denominator)
			if candidate.Cmp(&t.delta) < 0 {
				t.delta.Set(&candidate)
			}
		}
	}
	return nil
}
func (t *SMTDenseDiffTheory) ModelValue(node *SMTEnode, generator *SMTModelGenerator) (SMTModelValueProcedure, error) {
	if node == nil || generator == nil || generator.manager != t.state.manager {
		return nil, ast.ErrASTDomain
	}
	v := node.TheoryVar(t.FamilyID())
	if v == SMTNullTheoryVar || int(v) >= len(t.values) {
		return nil, ast.ErrASTDomain
	}
	var epsilon, value big.Rat
	epsilon.Mul(&t.delta, &t.values[v].infinitesimal)
	value.Add(&t.values[v].rational, &epsilon)
	expression, err := t.state.manager.ArithmeticNumeralRat(&value, t.integer[v])
	if err != nil {
		return nil, err
	}
	return NewSMTExpressionValueProcedure(expression)
}
