package smtcore

// Source: retained immediate-registration path in src/smt/theory_lra.cpp.
// The constructor-only m_delay_constraints flag has no retained writer.

import (
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/lp"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type smtLRABound struct {
	boolean  int32
	variable SMTTheoryVar
	value    big.Rat
	upper    bool
}

type smtLRAAssignment struct {
	boolean int32
	truth   bool
}

type smtLRAFrontendScope struct {
	bounds, assignments, head, underspecified int
	termBounds                                int
	assumptions, assumptionHead               int
	axiomRefs                                 int
	unassigned                                []uint32
	notHandled                                ast.ASTHandle
}

type smtLRAResourceLimit struct{ manager *ast.ASTManager }

func (l smtLRAResourceLimit) CancelFlag() (bool, error) { return l.manager.Canceled() }

// SMTLRATheory is the concrete frontend for the retained LAR-backed theory.
// Automatic QF_LRA/QF_UFLRA setup uses this backend. Remaining source trace
// sites and the complete retained-source audit are still incomplete.
type SMTLRATheory struct {
	*SMTTheoryBase
	validation                        *smtLRAValidationContext
	trace                             *diagnostic.TraceTags // Borrowed source trace control; nil uses the process tags.
	pendingTraceRoots                 []ast.ASTRef
	numeric                           *smtLRANumeric
	initParams                        *lp.LPParams
	numerals                          *numeral.RationalManager
	constructorInteger                *numeral.IntegerManager
	config                            SMTArithState
	equalityAdapter                   *smtLRAEqualityAdapter
	bounds                            [][]*smtLRABound
	boundByBoolean                    map[int32]*smtLRABound
	boundTrail                        []SMTTheoryVar
	newBounds                         []*smtLRABound
	boundChecks                       []int32
	propagator                        *lp.LARBoundPropagator
	basicPropagations, lpPropagations uint64
	conflictCount                     uint32
	statistics                        smtLRAStatistics
	lowerTerms, upperTerms            []smtLRAConstraintBound
	termBoundTrail                    []smtLRATermBoundChange
	fixedVariables                    map[smtLRAFixedKey]SMTTheoryVar
	fixedEqualities                   uint64
	assumeCandidates                  []smtArithmeticEqualityCandidate
	assumeHead                        int
	modelValues                       map[uint32]*big.Rat
	modelOwner                        *smtLRAModelOwner
	axiomManager                      *ast.ASTManager
	axiomRefs, pendingAxiomRoots      []ast.ASTRef
	unassigned                        []uint32
	assignments                       []smtLRAAssignment
	assignmentHead                    int
	scopes                            []smtLRAFrontendScope
	notHandled                        ast.ASTHandle
	underspecified                    []ast.ASTHandle
	closed                            bool
}

var _ SMTTheoryInternalizer = (*SMTLRATheory)(nil)

func NewSMTLRATheory(family ast.FamilyID) (*SMTLRATheory, error) {
	base, err := NewSMTTheoryBase(family)
	if err != nil {
		return nil, err
	}
	t := &SMTLRATheory{SMTTheoryBase: base,
		config:         SMTArithState{Reflect: true, BoundProp: 2, PropagateEqs: true, SmallLemmaSize: 16, PropagationThreshold: ^uint32(0)},
		boundByBoolean: make(map[int32]*smtLRABound)}
	t.fixedVariables = make(map[smtLRAFixedKey]SMTTheoryVar)
	t.equalityAdapter = newSMTLRAEqualityAdapter(t)
	return t, nil
}

func (t *SMTLRATheory) Initialize(s *SMTInternalizerState) (result error) {
	if t == nil || t.closed || t.SMTTheoryBase == nil {
		return ast.ErrASTHandle
	}
	if err := t.SMTTheoryBase.Initialize(s); err != nil {
		return err
	}
	defer func() {
		if result != nil && t.numeric != nil {
			t.numeric.stopped = true
		}
	}()
	t.axiomManager = s.manager
	var err error
	t.numerals, err = s.manager.NumeralService().NewPrivateRationalManager()
	if err != nil {
		// NewRationalManager can fail before publishing a rational owner.
		// Its error then carries the partial integer manager's physical roots.
		errors.As(err, &t.constructorInteger)
		return err
	}
	if s.smtParams != nil && s.contextParams != nil {
		t.initParams, err = lp.NewLPParams(s.smtParams.Context, s.contextParams, s.smtParams.Globals, s.manager.Budget())
		// Even a failed helper constructor can own a module reference. Keep
		// a refused destructor reachable until theory Close is retried.
		defer func() { result = errors.Join(result, t.closeInitParams()) }()
		if err != nil {
			return err
		}
	}
	t.numeric, err = newSMTLRANumeric(t.numerals, smtLRANumericConfig{
		strategy: lp.StrategyTableauRows, pivotedRows: true,
		boundPropagation: t.propagationEnabled(), limit: smtLRAResourceLimit{s.manager},
	})
	if err != nil || t.initParams == nil {
		return err
	}
	strategy, err := t.initParams.SimplexStrategy()
	if err != nil {
		return err
	}
	if strategy > uint32(lp.StrategyUndecided) {
		return ast.ErrASTDomain
	}
	t.numeric.solver.Settings.SimplexStrategy = lp.SimplexStrategy(strategy)
	pivoted, err := t.initParams.BpropOnPivotedRows()
	if err != nil {
		return err
	}
	t.numeric.solver.SetPropagateBoundsOnPivotedRowsMode(pivoted)
	return nil
}

func (t *SMTLRATheory) closeInitParams() error {
	if t.initParams != nil {
		if err := t.initParams.Close(); err != nil {
			return err
		}
		t.initParams = nil
	}
	return nil
}

func (t *SMTLRATheory) ConfigureArithmetic(config SMTArithState) error {
	if t == nil || t.closed || t.equalityAdapter == nil {
		return ast.ErrASTHandle
	}
	if config.LazyAdapter || config.AddBinaryBounds || config.EnumConstMod {
		// Lazy/constant-mod enumeration have no retained writer; the
		// binary-bound writer selects the QF_RDL backend.
		return ast.ErrASTDomain
	}
	// Source init_solver sets the LP row-tracking flag once. Live arithmetic
	// changes gate callbacks through propagation_mode; they do not reconfigure
	// an existing LAR solver or recreate rows that were never tracked.
	t.config = config
	return nil
}

func (t *SMTLRATheory) operation(h ast.ASTHandle) (int32, bool, error) {
	kind, err := t.state.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return 0, false, err
	}
	family, err := t.state.manager.GetFamilyID(h)
	if err != nil || family != t.FamilyID() {
		return 0, false, err
	}
	op, err := t.state.manager.GetDeclKind(h)
	return op, err == nil, err
}

func (t *SMTLRATheory) numeral(h ast.ASTHandle) (*big.Rat, bool, error) {
	text, _, matched, err := t.state.manager.ArithmeticNumeralText(h)
	if err != nil || !matched {
		return nil, false, err
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, false, ast.ErrASTDomain
	}
	return value, true, nil
}

func (t *SMTLRATheory) underspecifiedTerm(h ast.ASTHandle) (bool, error) {
	op, own, err := t.operation(h)
	return own && (op == smtArithOpDiv || op == smtArithOpIDiv || op == smtArithOpRem || op == smtArithOpMod), err
}

func (t *SMTLRATheory) foundNotHandled(h ast.ASTHandle) error {
	t.notHandled = h
	u, err := t.underspecifiedTerm(h)
	if err == nil && u {
		t.underspecified = append(t.underspecified, h)
	}
	if err != nil {
		return err
	}
	return t.traceLRAExpression("found_not_handled", 300, "Unhandled: ", h, "\n")
}

func (t *SMTLRATheory) reflect(h ast.ASTHandle) (bool, error) {
	if t.config.Reflect {
		return true, nil
	}
	return t.underspecifiedTerm(h)
}

func (t *SMTLRATheory) internalizeArguments(h ast.ASTHandle) error {
	reflect, err := t.reflect(h)
	if err != nil || !reflect {
		return err
	}
	count, err := t.state.manager.GetNumArgs(h)
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		arg, err := t.state.manager.GetArg(h, i)
		if err != nil {
			return err
		}
		if t.state.context.enodes[arg] == nil {
			if err = t.state.Internalize(arg, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *SMTLRATheory) mkEnode(h ast.ASTHandle) (*SMTEnode, error) {
	if node := t.state.context.enodes[h]; node != nil {
		return node, nil
	}
	reflect, err := t.reflect(h)
	if err != nil {
		return nil, err
	}
	op, own, err := t.operation(h)
	if err != nil {
		return nil, err
	}
	return t.state.MkEnode(h, !reflect, false, !(own && (op == smtArithOpAdd || op == smtArithOpMul)))
}

func (t *SMTLRATheory) mkVar(h ast.ASTHandle) (SMTTheoryVar, error) {
	if t.state.context.enodes[h] == nil {
		if err := t.state.Internalize(h, false); err != nil {
			return SMTNullTheoryVar, err
		}
	}
	node := t.state.context.enodes[h]
	if node == nil {
		return SMTNullTheoryVar, ast.ErrASTDomain
	}
	v := node.TheoryVar(t.FamilyID())
	if !t.IsAttachedToVariable(node) {
		// Source th.mk_var publishes the borrowed enode slot first; context
		// attachment follows the bounds invariant and directory extension.
		v = SMTTheoryVar(len(t.variables))
		t.variables = append(t.variables, node)
		if d, enabled, err := lraAssertionContext(); err != nil {
			return SMTNullTheoryVar, err
		} else if enabled && !(len(t.bounds) <= int(v) || len(t.bounds[v]) == 0) {
			return SMTNullTheoryVar, diagnostic.DebugAssertionFailure(d, "src/smt/theory_lra.cpp", 476, "m_bounds.size() <= static_cast<unsigned>(v) || m_bounds[v].empty()")
		}
		for len(t.bounds) <= int(v) {
			t.bounds = append(t.bounds, nil)
			t.unassigned = append(t.unassigned, 0)
		}
		if err := t.state.attachTheoryVariable(node, t.FamilyID(), v); err != nil {
			// The allocated source variable remains owned until pop/Close.
			t.numeric.stopped = true
			return SMTNullTheoryVar, err
		}
	}
	if d, enabled, err := lraAssertionContext(); err != nil {
		return SMTNullTheoryVar, err
	} else if enabled && v == SMTNullTheoryVar {
		return SMTNullTheoryVar, diagnostic.DebugAssertionFailure(d, "src/smt/theory_lra.cpp", 486, "null_theory_var != v")
	}
	return v, nil
}

type smtLRALinearization struct {
	terms         []ast.ASTHandle
	coefficients  []big.Rat
	variables     []SMTTheoryVar
	constant      big.Rat
	toInternalize []ast.ASTHandle
}

func (s *smtLRALinearization) push(h ast.ASTHandle, coefficient *big.Rat) {
	s.terms = append(s.terms, h)
	s.coefficients = append(s.coefficients, *new(big.Rat).Set(coefficient))
}

func (s *smtLRALinearization) setBack(index int) {
	last := len(s.terms) - 1
	if index == last {
		return
	}
	s.terms[index] = s.terms[last]
	s.coefficients[index].Set(&s.coefficients[last])
	s.terms = s.terms[:last]
	s.coefficients = s.coefficients[:last]
}

// Source linearize's producer-order work stack, including swap-with-last
// removal. Constant compaction precedes variable coefficient compaction.
func (t *SMTLRATheory) linearize(term ast.ASTHandle) (smtLRALinearization, error) {
	var st smtLRALinearization
	st.push(term, big.NewRat(1, 1))
	for index := 0; index < len(st.terms); {
		if d, enabled, err := lraAssertionContext(); err != nil {
			return st, err
		} else if enabled && index < len(st.variables) {
			return st, diagnostic.DebugAssertionFailure(d, "src/smt/theory_lra.cpp", 343, "index >= vars.size()")
		}
		h := st.terms[index]
		st.toInternalize = append(st.toInternalize, h)
		op, own, err := t.operation(h)
		if err != nil {
			return st, err
		}
		count, err := t.state.manager.GetNumArgs(h)
		if err != nil {
			return st, err
		}
		arg := func(i uint32) (ast.ASTHandle, error) { return t.state.manager.GetArg(h, i) }
		switch {
		case own && op == smtArithOpAdd:
			coefficient := new(big.Rat).Set(&st.coefficients[index])
			for i := uint32(0); i < count; i++ {
				a, e := arg(i)
				if e != nil {
					return st, e
				}
				st.push(a, coefficient)
			}
			st.setBack(index)
		case own && op == smtArithOpSub:
			if count == 0 {
				return st, ast.ErrASTDomain
			}
			first, e := arg(0)
			if e != nil {
				return st, e
			}
			st.terms[index] = first
			negative := new(big.Rat).Neg(&st.coefficients[index])
			for i := uint32(1); i < count; i++ {
				a, e := arg(i)
				if e != nil {
					return st, e
				}
				st.push(a, negative)
			}
		case own && op == smtArithOpUMinus:
			if count != 1 {
				return st, ast.ErrASTDomain
			}
			first, e := arg(0)
			if e != nil {
				return st, e
			}
			st.terms[index] = first
			st.coefficients[index].Neg(&st.coefficients[index])
		case own && op == smtArithOpNum:
			r, matched, e := t.numeral(h)
			if e != nil {
				return st, e
			}
			if !matched {
				return st, ast.ErrASTDomain
			}
			st.constant.Add(&st.constant, new(big.Rat).Mul(&st.coefficients[index], r))
			index++
		default:
			if own && op == smtArithOpMul && count == 2 {
				first, e := arg(0)
				if e != nil {
					return st, e
				}
				second, e := arg(1)
				if e != nil {
					return st, e
				}
				// Native is_numeral has a single do/while(false) pass. Its
				// unary-minus/to_real branches continue out of the loop and
				// return false; only an actual numeral scales a product here.
				coefficient, matched, e := t.numeral(first)
				if e != nil {
					return st, e
				}
				factor, numeral := second, first
				if !matched {
					coefficient, matched, e = t.numeral(second)
					factor, numeral = first, second
					if e != nil {
						return st, e
					}
				}
				if matched {
					st.coefficients[index].Mul(&st.coefficients[index], coefficient)
					st.terms[index] = factor
					st.toInternalize = append(st.toInternalize, numeral)
					continue
				}
			}
			if own {
				division := false
				if op == smtArithOpDiv && count == 2 {
					denominator, e := arg(1)
					if e != nil {
						return st, e
					}
					_, division, err = t.numeral(denominator)
					if err != nil {
						return st, err
					}
				}
				if !division {
					if err = t.foundNotHandled(h); err != nil {
						return st, err
					}
				}
				if err = t.internalizeArguments(h); err != nil {
					return st, err
				}
				if _, err = t.mkEnode(h); err != nil {
					return st, err
				}
			} else {
				if err = t.internalizeArguments(h); err != nil {
					return st, err
				}
				sort, e := t.state.manager.GetSort(h)
				if e != nil {
					return st, e
				}
				family, e := t.state.manager.GetFamilyID(sort)
				if e != nil {
					return st, e
				}
				if family == t.FamilyID() {
					kind, e := t.state.manager.GetDeclKind(sort)
					if e != nil {
						return st, e
					}
					if kind == 1 {
						if e = t.foundNotHandled(h); e != nil {
							return st, e
						}
					}
				}
			}
			v, e := t.mkVar(h)
			if e != nil {
				return st, e
			}
			st.coefficients[len(st.variables)].Set(&st.coefficients[index])
			st.variables = append(st.variables, v)
			index++
		}
	}
	for i := len(st.toInternalize); i > 0; i-- {
		if _, err := t.mkEnode(st.toInternalize[i-1]); err != nil {
			return st, err
		}
	}
	return st, nil
}

func (t *SMTLRATheory) internalizeDef(term ast.ASTHandle) (SMTTheoryVar, error) {
	st, err := t.linearize(term)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if st.constant.Sign() == 0 && len(st.variables) == 1 && st.coefficients[0].Cmp(big.NewRat(1, 1)) == 0 {
		return st.variables[0], nil
	}
	v, err := t.mkVar(term)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	coefficients := make([]smtLRACoefficient, len(st.variables))
	for i, w := range st.variables {
		coefficients[i].variable = w
		coefficients[i].value.Set(&st.coefficients[i])
	}
	_, wasMapped := t.numeric.index(v)
	index, err := t.numeric.addTerm(v, coefficients, &st.constant)
	if err == nil && !wasMapped {
		err = t.emitLRA("internalize_def", 640, func(out io.Writer) error {
			if err := smtWriteTheoryText(out, fmt.Sprintf("v%d := ", v)); err != nil {
				return err
			}
			if err := t.displayLRAExpression(out, term); err != nil {
				return err
			}
			if err := smtWriteTheoryText(out, fmt.Sprintf(" slack: %d scopes: %d\n", index, len(t.scopes))); err != nil {
				return err
			}
			if !t.numeric.solver.IsTerm(index) || uint64(index-lp.LARTermStartIndex) >= uint64(len(t.numeric.solver.Terms)) {
				return ast.ErrASTDomain
			}
			if err := t.numeric.solver.PrintTerm(t.numeric.solver.Terms[index-lp.LARTermStartIndex], out); err != nil {
				return err
			}
			return smtWriteTheoryText(out, "\n")
		})
	}
	if err == nil {
		if value, number, e := t.numeral(term); e != nil {
			return v, e
		} else if number {
			key, e := t.fixedKey(v, value)
			if e != nil {
				return v, e
			}
			t.fixedVariables[key] = v
		}
	}
	return v, err
}

func (t *SMTLRATheory) InternalizeTerm(term ast.ASTHandle) (bool, error) {
	if _, err := t.State(); err != nil {
		return false, err
	}
	if node := t.state.context.enodes[term]; node != nil && t.IsAttachedToVariable(node) {
		return true, nil
	}
	_, err := t.internalizeDef(term)
	return err == nil, err
}

func (t *SMTLRATheory) InternalizeAtom(atom ast.ASTHandle, _ bool) (bool, error) {
	if _, err := t.State(); err != nil {
		return false, err
	}
	if d, enabled, err := lraAssertionContext(); err != nil {
		return false, err
	} else if enabled {
		if _, found := t.state.BoolVar(atom); found {
			return false, diagnostic.DebugAssertionFailure(d, "src/smt/theory_lra.cpp", 689, "!ctx().b_internalized(atom)")
		}
	}
	bv, err := t.state.MkBoolVar(atom)
	if err != nil {
		return false, err
	}
	if err = t.state.SetVariableTheory(bv, t.FamilyID()); err != nil {
		return false, err
	}
	op, own, err := t.operation(atom)
	if err != nil {
		return false, err
	}
	if !own || op != smtArithOpLE && op != smtArithOpGE {
		return t.refuseAtom(atom)
	}
	lhs, err := t.state.manager.GetArg(atom, 0)
	if err != nil {
		return false, err
	}
	rhs, err := t.state.manager.GetArg(atom, 1)
	if err != nil {
		return false, err
	}
	r, matched, err := t.numeral(rhs)
	if err != nil {
		return false, err
	}
	kind, err := t.state.manager.GetKind(lhs)
	if err != nil {
		return false, err
	}
	if !matched || kind != ast.ASTApp {
		return t.refuseAtom(atom)
	}
	v, err := t.internalizeDef(lhs)
	if err != nil {
		return false, err
	}
	b := &smtLRABound{boolean: bv, variable: v, upper: op == smtArithOpLE}
	b.value.Set(r)
	t.bounds[v] = append(t.bounds[v], b)
	if err = t.updateUnassignedBounds(v, 1); err != nil {
		return false, err
	}
	t.boundTrail = append(t.boundTrail, v)
	t.boundByBoolean[bv] = b
	if err = t.traceLRAExpression("internalize_atom_strict", 714, "Internalized ", atom, "\n"); err != nil {
		return false, err
	}
	if err = t.mkBoundAxioms(b); err != nil {
		t.newBounds = append(t.newBounds, b)
		return false, err
	}
	return true, nil
}

// LRA does not override native apply_sort_cnstr; foreign terms acquire a
// variable only when the LRA linearizer reaches mk_var.
func (t *SMTLRATheory) ApplySortConstraint(_ *SMTEnode, _ ast.ASTHandle) error {
	_, err := t.State()
	return err
}

func (t *SMTLRATheory) InternalizeEquality(h ast.ASTHandle, _ int32) error {
	if _, err := t.State(); err != nil {
		return err
	}
	left, err := t.state.manager.GetArg(h, 0)
	if err != nil {
		return err
	}
	right, err := t.state.manager.GetArg(h, 1)
	if err != nil {
		return err
	}
	n1, n2 := t.state.context.enodes[left], t.state.context.enodes[right]
	if n1 != nil && n2 != nil && n1 != n2 && t.IsAttachedToVariable(n1) && t.IsAttachedToVariable(n2) {
		if err = t.traceLRAExpression("internalize_eq_eh", 781, "", h, "\n"); err != nil {
			return err
		}
		return t.equalityAdapter.MkAxioms(n1.TheoryVar(t.FamilyID()), n2.TheoryVar(t.FamilyID()))
	}
	return nil
}
func (t *SMTLRATheory) NewEquality(left, right SMTTheoryVar) error {
	if _, err := t.State(); err != nil {
		return err
	}
	return t.equalityAdapter.NewEquality(left, right)
}
func (t *SMTLRATheory) NewDisequality(left, right SMTTheoryVar) error {
	if _, err := t.State(); err != nil {
		return err
	}
	if err := t.emitLRA("new_diseq_eh", 806, func(out io.Writer) error {
		return smtWriteTheoryText(out, fmt.Sprintf("v%d != v%d\n", left, right))
	}); err != nil {
		return err
	}
	t.statistics.disequalities++
	return t.equalityAdapter.NewDisequality(left, right)
}
func (t *SMTLRATheory) InitializeSearch() error {
	if _, err := t.State(); err != nil {
		return err
	}
	if err := t.equalityAdapter.InitializeSearch(); err != nil {
		return err
	}
	t.conflictCount = 0
	return nil
}
func (t *SMTLRATheory) Restart() error {
	if _, err := t.State(); err != nil {
		return err
	}
	return t.equalityAdapter.Restart()
}

func (t *SMTLRATheory) Assign(bv int32, truth bool) error {
	if _, err := t.State(); err != nil {
		return err
	}
	if t.boundByBoolean[bv] == nil {
		return ast.ErrASTDomain
	}
	if err := t.emitLRA("assign_eh", 787, func(out io.Writer) error {
		expression, err := t.state.BoolExpression(bv)
		if err != nil {
			return err
		}
		if err = t.displayLRAExpression(out, expression); err != nil {
			return err
		}
		return smtWriteTheoryText(out, fmt.Sprintf(" %t\n", truth))
	}); err != nil {
		return err
	}
	t.assignments = append(t.assignments, smtLRAAssignment{bv, truth})
	return nil
}
func (t *SMTLRATheory) CanPropagate() bool {
	return t != nil && !t.closed && t.assignmentHead < len(t.assignments)
}
func (t *SMTLRATheory) Propagate() error {
	if _, err := t.State(); err != nil {
		return err
	}
	if err := t.numeric.ready(); err != nil {
		return err
	}
	if t.propagator != nil {
		return lp.ErrExactVectorPending
	}
	if err := t.flushBoundAxioms(); err != nil {
		return err
	}
	if !t.CanPropagate() {
		return nil
	}
	for t.assignmentHead < len(t.assignments) && !t.state.inconsistent {
		a := t.assignments[t.assignmentHead]
		b := t.boundByBoolean[a.boolean]
		if b == nil {
			return ast.ErrASTDomain
		}
		t.boundChecks = append(t.boundChecks, a.boolean)
		ci, err := t.numeric.addBound(b.variable, &b.value, b.upper, a.truth)
		if err != nil {
			return err
		}
		if ci != smtLRANoIndex {
			if err = t.emitLRA("assert_bound", 1973, func(out io.Writer) error { return smtWriteTheoryText(out, fmt.Sprintf("v%d\n", b.variable)) }); err != nil {
				return err
			}
			t.numeric.registerBoundConstraint(ci, a.boolean, a.truth)
			if err = t.emitLRA("add_ineq_constraint", 550, func(out io.Writer) error {
				if err := t.numeric.solver.PrintConstraint(ci, out); err != nil {
					return err
				}
				return smtWriteTheoryText(out, "\n")
			}); err != nil {
				return err
			}
		}
		if err = t.propagateFixedBounds(b, ci, a.truth); err != nil {
			t.numeric.stopped = true
			return err
		}
		t.assignmentHead++
	}
	if t.state.inconsistent {
		t.boundChecks = nil
		return nil
	}
	status, err := t.makeFeasible()
	if err != nil {
		return err
	}
	if status == lbool.False {
		if err := t.emitLRA("propagate", 1290, func(out io.Writer) error { return smtWriteTheoryText(out, "propagation conflict\n") }); err != nil {
			return err
		}
		return t.setLRAConflict()
	}
	if status == lbool.True {
		if err := t.propagateBasicBounds(); err != nil {
			t.numeric.stopped = true
			return err
		}
		return t.propagateLPBounds()
	}
	return nil
}

func (t *SMTLRATheory) FinalCheck() (SMTFinalCheckStatus, error) {
	if _, err := t.State(); err != nil {
		return SMTFinalCheckGiveUp, err
	}
	if err := t.numeric.ready(); err != nil {
		return SMTFinalCheckGiveUp, err
	}
	status := lbool.True
	if t.numeric.solver.Bounds.Status.Value != lp.StatusOptimal {
		var err error
		status, err = t.makeFeasible()
		if err != nil {
			return SMTFinalCheckGiveUp, err
		}
	}
	switch status {
	case lbool.True:
		if progress, err := t.delayedAssumeEqualities(); err != nil || progress {
			return SMTFinalCheckContinue, err
		}
		if progress, err := t.assumeSharedEqualities(); err != nil || progress {
			return SMTFinalCheckContinue, err
		}
		if !t.notHandled.IsNull() {
			return SMTFinalCheckGiveUp, nil
		}
		return SMTFinalCheckDone, nil
	case lbool.False:
		return SMTFinalCheckContinue, t.setLRAConflict()
	default:
		canceled, err := t.state.manager.Canceled()
		if canceled {
			return SMTFinalCheckContinue, err
		}
		return SMTFinalCheckGiveUp, err
	}
}

func (t *SMTLRATheory) PushScope() error {
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	t.scopes = append(t.scopes, smtLRAFrontendScope{bounds: len(t.boundTrail), assignments: len(t.assignments), head: t.assignmentHead, underspecified: len(t.underspecified), termBounds: len(t.termBoundTrail), assumptions: len(t.assumeCandidates), assumptionHead: t.assumeHead, axiomRefs: len(t.axiomRefs), notHandled: t.notHandled, unassigned: append([]uint32(nil), t.unassigned...)})
	if err := t.equalityAdapter.PushScope(); err != nil {
		return err
	}
	return t.numeric.push()
}
func (t *SMTLRATheory) PopScopes(n uint32) error {
	if _, err := t.State(); err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	if int(n) > len(t.scopes) {
		return ast.ErrASTDomain
	}
	level := len(t.scopes) - int(n)
	scope := t.scopes[level]
	for len(t.axiomRefs) > scope.axiomRefs {
		i := len(t.axiomRefs) - 1
		if err := t.axiomManager.DelRef(&t.axiomRefs[i]); err != nil {
			return err
		}
		t.axiomRefs = t.axiomRefs[:i]
	}
	for i := len(t.boundTrail); i > scope.bounds; i-- {
		v := t.boundTrail[i-1]
		last := len(t.bounds[v]) - 1
		b := t.bounds[v][last]
		delete(t.boundByBoolean, b.boolean)
		t.bounds[v][last] = nil
		t.bounds[v] = t.bounds[v][:last]
	}
	t.boundTrail = t.boundTrail[:scope.bounds]
	t.assignments = t.assignments[:scope.assignments]
	t.assignmentHead = scope.head
	t.notHandled = scope.notHandled
	t.underspecified = t.underspecified[:scope.underspecified]
	copy(t.unassigned, scope.unassigned)
	clear(t.unassigned[len(scope.unassigned):])
	t.scopes = t.scopes[:level]
	t.newBounds = nil // Native user-push contract flushes pending parent axioms first.
	t.boundChecks = nil
	t.assumeCandidates = t.assumeCandidates[:scope.assumptions]
	t.assumeHead = scope.assumptionHead
	for len(t.termBoundTrail) > scope.termBounds {
		i := len(t.termBoundTrail) - 1
		entry := t.termBoundTrail[i]
		bounds := t.upperTerms
		if entry.lower {
			bounds = t.lowerTerms
		}
		bounds[entry.index] = entry.previous
		t.termBoundTrail = t.termBoundTrail[:i]
	}
	if err := t.numeric.pop(n); err != nil {
		return err
	}
	if err := t.equalityAdapter.PopScopes(n); err != nil {
		return err
	}
	if err := t.SMTTheoryBase.PopScopes(n); err != nil {
		return err
	}
	return t.emitLRA("pop_scope_eh", 856, func(out io.Writer) error {
		return smtWriteTheoryText(out, fmt.Sprintf("num scopes: %d new scope level: %d\n", n, len(t.scopes)))
	})
}

func (t *SMTLRATheory) Close() error {
	if t == nil {
		return nil
	}
	// A refused child cleanup still owns its state, but the frontend must
	// already be unavailable to ordinary calls while Close is retried.
	t.closed = true
	t.state = nil
	if err := t.closeValidation(); err != nil {
		return err
	}
	if err := t.closeInitParams(); err != nil {
		return err
	}
	if t.constructorInteger != nil {
		if err := t.constructorInteger.CloseChecked(); err != nil {
			return err
		}
		t.constructorInteger = nil
	}
	for _, refs := range []*[]ast.ASTRef{&t.pendingTraceRoots, &t.pendingAxiomRoots, &t.axiomRefs} {
		for len(*refs) > 0 {
			i := len(*refs) - 1
			if err := t.axiomManager.DelRef(&(*refs)[i]); err != nil {
				return err
			}
			*refs = (*refs)[:i]
		}
	}
	if t.propagator != nil {
		if err := t.propagator.Close(); err != nil {
			return err
		}
		t.propagator = nil
	}
	if t.numeric != nil {
		if err := t.numeric.Close(); err != nil {
			return err
		}
		t.numeric = nil
	}
	if t.numerals != nil {
		if err := t.numerals.CloseChecked(); err != nil {
			return err
		}
		t.numerals = nil
	}
	if t.equalityAdapter != nil {
		if err := t.equalityAdapter.Close(); err != nil {
			return err
		}
		t.equalityAdapter = nil
	}
	t.variables = nil
	t.limits = nil
	t.bounds = nil
	t.boundByBoolean = nil
	t.boundTrail = nil
	t.newBounds = nil
	t.boundChecks = nil
	t.lowerTerms = nil
	t.upperTerms = nil
	t.termBoundTrail = nil
	t.fixedVariables = nil
	t.assumeCandidates = nil
	t.modelValues = nil
	t.modelOwner = nil
	t.unassigned = nil
	t.assignments = nil
	t.scopes = nil
	t.underspecified = nil
	return nil
}

func (t *SMTLRATheory) refuseAtom(atom ast.ASTHandle) (bool, error) {
	if err := t.traceLRAExpression("internalize_atom_strict", 705, "Could not internalize ", atom, "\n"); err != nil {
		return false, err
	}
	return true, t.foundNotHandled(atom)
}
