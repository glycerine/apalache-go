package sat_solver

// Source: src/sat/sat_solver/inc_sat_solver.cpp assertion/check/result paths.

import (
	"errors"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

type solverScope struct {
	formulas, assumptions int
	head                  uint32
}
type converterOwner struct {
	value   tactic.ModelConverter
	cleanup bool
}
type checkWork struct {
	owner       *Solver
	assumptions []ast.ASTHandle
	held        []ast.ASTHandle
	original    map[ast.ASTHandle]ast.ASTHandle
}

// Solver implements the retained incremental SAT state machine, including
// models, consequences, diagnostics and weighted output. Evaluator services
// are borrowed. A nonnil partial constructor result must close.
type Solver struct {
	self                         *Solver
	manager                      *ast.ASTManager
	services                     model.ModelEvaluatorConfig
	output                       io.Writer
	dimacs                       *utext.NumericTextStream
	targetServices               func(*ast.ASTManager) (model.ModelEvaluatorConfig, error)
	runtime                      *sat.Runtime
	core                         *sat.Solver
	encoder                      *IncrementalEncoder
	baseParams, internalParams   uparams.ParamsRef
	baseReady, internalReady     bool
	formulas, tracked, unsatCore []ast.ASTHandle
	consequences                 []ast.ASTHandle
	head                         uint32
	scopes                       []solverScope
	assumptions                  []sat.Literal
	weights                      []uint32
	model                        tactic.ModelRef
	mc0                          *converterOwner
	converters                   []*converterOwner
	modelWork                    []*modelConversion
	modelPending                 bool
	lastGoal                     *tactic.Goal
	goals                        []*tactic.Goal
	checks                       []*checkWork
	translations                 []*solverTranslation
	failures                     []memory.ConstructionFailure
	status                       lbool.LBool
	unknown                      string
	refs                         uint32
	header                       uint64
	closed                       bool
}

type IncrementalSolverConfig struct {
	Services       model.ModelEvaluatorConfig
	TargetServices func(*ast.ASTManager) (model.ModelEvaluatorConfig, error)
	Output         io.Writer
}

func NewIncrementalSolver(m *ast.ASTManager, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*Solver, error) {
	return NewIncrementalSolverWithConfig(m, p, IncrementalSolverConfig{Services: services})
}

func NewIncrementalSolverWithConfig(m *ast.ASTManager, p *uparams.ParamsRef, config IncrementalSolverConfig) (*Solver, error) {
	services := config.Services
	c, g := services.Context, services.Globals
	if m == nil || c == nil || g == nil || g.Context != c || services.Memory == nil || services.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(Solver{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	output := config.Output
	if output == nil {
		output = os.Stdout
	}
	s := &Solver{manager: m, services: services, output: output, targetServices: config.TargetServices, header: n, unknown: "no reason given", status: lbool.Undef}
	s.self = s
	var err error
	s.baseParams, err = c.NewValue()
	s.baseReady = err == nil
	if err == nil && p != nil {
		err = c.Assign(&s.baseParams, p)
	}
	if err == nil {
		s.internalParams, err = c.NewValue()
		s.internalReady = err == nil
	}
	if err == nil {
		s.runtime, err = sat.NewRuntime(sat.RuntimeConfig{Budget: m.Budget(), ParamsContext: c, Globals: services.Globals, Process: m.NumeralService().RuntimeOwner().ProcessOwner()})
	}
	if err == nil {
		s.core, err = sat.NewSATSolver(sat.SolverConfig{ParamsContext: c, Params: s.baseParams, RLimit: m.ResourceLimit(), Budget: m.Budget(), Runtime: s.runtime, TraceTags: services.Trace})
	}
	if err == nil {
		err = s.UpdateParams(p)
	}
	if err == nil {
		s.encoder, err = NewIncrementalEncoder(m, s.core, &s.internalParams, services)
	}
	s.retainFailure(err)
	return s, err
}
func (s *Solver) retainFailure(err error) {
	var failure memory.ConstructionFailure
	if errors.As(err, &failure) {
		s.failures = append(s.failures, failure)
	}
}
func (s *Solver) open() error {
	if s == nil || s.self != s {
		return ast.ErrASTHandle
	}
	if s.closed {
		return arena.ErrClosed
	}
	if !s.baseReady || !s.internalReady || s.core == nil {
		return ast.ErrASTPending
	}
	return s.core.CheckOwner()
}
func releaseRoots(m *ast.ASTManager, values *[]ast.ASTHandle, size int) error {
	for i := size; i < len(*values); i++ {
		consumed, err := m.DecRefConsumed((*values)[i])
		if consumed {
			(*values)[i] = ast.ASTHandle{}
		}
		if err != nil {
			return err
		}
	}
	*values = (*values)[:size]
	return nil
}
func (s *Solver) hold(values *[]ast.ASTHandle, h ast.ASTHandle) error {
	if err := s.manager.IncRef(h); err != nil {
		return err
	}
	*values = append(*values, h)
	return nil
}
func (s *Solver) drainGoals() error {
	for len(s.goals) > 0 {
		i := len(s.goals) - 1
		if err := s.goals[i].Close(); err != nil {
			return err
		}
		s.goals = s.goals[:i]
	}
	return nil
}
func (s *Solver) drainChecks() error {
	for len(s.checks) > 0 {
		i := len(s.checks) - 1
		if err := releaseRoots(s.manager, &s.checks[i].held, 0); err != nil {
			return err
		}
		s.checks = s.checks[:i]
	}
	return nil
}
func (s *Solver) drainConverters() error {
	for len(s.converters) > 0 {
		i := len(s.converters) - 1
		o := s.converters[i]
		if o.cleanup {
			if err := o.value.Close(); err != nil {
				return err
			}
		} else {
			consumed, err := o.value.DecRefConsumed()
			if consumed && err != nil {
				o.cleanup = true
			}
			if err != nil {
				return err
			}
		}
		s.converters = s.converters[:i]
	}
	return nil
}
func (s *Solver) Close() error {
	if s == nil || s.self != s {
		return ast.ErrASTHandle
	}
	if s.closed {
		return nil
	}
	if err := s.drainTranslations(); err != nil {
		return err
	}
	for len(s.failures) > 0 {
		i := len(s.failures) - 1
		if err := s.failures[i].Cleanup(); err != nil {
			return err
		}
		s.failures = s.failures[:i]
	}
	if err := s.drainChecks(); err != nil {
		return err
	}
	if err := s.drainModelWork(); err != nil {
		return err
	}
	if err := s.model.Close(); err != nil {
		return err
	}
	if s.mc0 != nil {
		s.converters = append(s.converters, s.mc0)
		s.mc0 = nil
	}
	if err := s.drainConverters(); err != nil {
		return err
	}
	if s.encoder != nil {
		if err := s.encoder.Close(); err != nil {
			return err
		}
		s.encoder = nil
	}
	if s.lastGoal != nil {
		s.goals = append(s.goals, s.lastGoal)
		s.lastGoal = nil
	}
	if err := s.drainGoals(); err != nil {
		return err
	}
	for _, roots := range []*[]ast.ASTHandle{&s.consequences, &s.unsatCore, &s.tracked, &s.formulas} {
		if err := releaseRoots(s.manager, roots, 0); err != nil {
			return err
		}
	}
	if s.core != nil {
		if err := s.core.Close(); err != nil {
			if s.core.StoppedCleanupCause() != nil {
				return &CleanupFailure{owner: s, cause: err}
			}
			return err
		}
		s.core = nil
	}
	if s.dimacs != nil {
		if err := s.dimacs.Close(); err != nil {
			return err
		}
		s.dimacs = nil
	}
	if s.runtime != nil {
		if err := s.runtime.Close(); err != nil {
			return err
		}
		s.runtime = nil
	}
	if s.internalReady {
		if err := s.services.Context.Del(&s.internalParams); err != nil {
			return err
		}
		s.internalReady = false
	}
	if s.baseReady {
		if err := s.services.Context.Del(&s.baseParams); err != nil {
			return err
		}
		s.baseReady = false
	}
	s.manager.Budget().Release(s.header)
	s.header = 0
	s.closed = true
	return nil
}
func (s *Solver) IncRef() error {
	if err := s.open(); err != nil {
		return err
	}
	s.refs++
	return nil
}
func (s *Solver) DecRef() error {
	if s == nil || s.self != s {
		return ast.ErrASTHandle
	}
	if s.closed {
		return arena.ErrClosed
	}
	if s.refs == 0 {
		return arena.ErrBounds
	}
	s.refs--
	if s.refs == 0 {
		return s.Close()
	}
	return nil
}
func (s *Solver) Manager() (*ast.ASTManager, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return s.manager, nil
}
func (s *Solver) Status() (lbool.LBool, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	return s.status, nil
}
func (s *Solver) SetStatus(v lbool.LBool) error {
	if err := s.open(); err != nil {
		return err
	}
	s.status = v
	return nil
}
func (s *Solver) ReasonUnknown() (string, error) {
	if err := s.open(); err != nil {
		return "", err
	}
	return s.unknown, nil
}
func (s *Solver) SetReasonUnknown(v string) error {
	if err := s.open(); err != nil {
		return err
	}
	s.unknown, _, _ = strings.Cut(v, "\x00")
	return nil
}
func (s *Solver) Labels() ([]usymbol.Symbol, error) { return nil, s.open() }
func (s *Solver) Proof() (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if ast.ASTSourceDebug {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return ast.ASTHandle{}, nil
}
func (s *Solver) Params() (*uparams.ParamsRef, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return &s.baseParams, nil
}
func (s *Solver) UpdateParams(p *uparams.ParamsRef) error {
	if err := s.open(); err != nil {
		return err
	}
	c := s.services.Context
	if p == nil {
		empty := c.EmptyValue()
		p = &empty
	}
	// The derived source m_params shadows solver::m_params. Preserve both:
	// user settings update the base snapshot; the SAT core receives only the
	// internal dictionary with variable elimination disabled.
	if err := c.Assign(&s.baseParams, p); err != nil {
		return err
	}
	key := "elim_vars"
	if err := c.SetBoolName(&s.internalParams, &key, false); err != nil {
		return err
	}
	return s.core.UpdateParams(&s.internalParams)
}
func (s *Solver) SetProduceModels(bool) error                       { return s.open() } // Source no-op.
func (s *Solver) SetProgressCallback(solver.ProgressCallback) error { return s.open() }
func (s *Solver) CollectParamDescrs(d *uparams.ParamDescrs) error {
	if err := s.open(); err != nil {
		return err
	}
	if err := uparams.InsertMaxMemory(d); err != nil {
		return err
	}
	description := "(default: true) add redundant clauses (that improve unit propagation) when encoding if-then-else formulas"
	if err := d.InsertName("ite_extra", uparams.ParamBool, &description, nil, nil); err != nil {
		return err
	}
	return sat.CollectSolverParamDescrs(d)
}
func (s *Solver) CollectStatistics(st *statistics.Statistics) error {
	if err := s.open(); err != nil {
		return err
	}
	if s.encoder != nil && s.encoder.pipeline != nil {
		for k, v := range s.encoder.pipeline.root.CollectStatistics() {
			st.UpdateDouble(statistics.Key(k), v)
		}
	}
	return s.core.CollectStatistics(st)
}
func (s *Solver) Assert(h ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	if err := s.emit("assert_expr", 231, func(out io.Writer) error {
		if err := s.writeExpression(out, h); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	}); err != nil {
		return err
	}
	return s.hold(&s.formulas, h)
}
func (s *Solver) AssertTracked(h, a ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	if a.IsNull() {
		return s.Assert(h)
	}
	if err := s.hold(&s.tracked, a); err != nil {
		return err
	}
	f, err := s.manager.MkImplies(a, h)
	if err != nil {
		return err
	}
	return s.Assert(f)
}
func (s *Solver) NumAssertions() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return uint32(len(s.formulas)), nil
}
func (s *Solver) Assertion(i uint32) (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if uint64(i) >= uint64(len(s.formulas)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return s.formulas[i], nil
}
func (s *Solver) NumAssumptions() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return uint32(len(s.tracked)), nil
}
func (s *Solver) Assumption(i uint32) (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if uint64(i) >= uint64(len(s.tracked)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return s.tracked[i], nil
}
func (s *Solver) ScopeLevel() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return uint32(len(s.scopes)), nil
}
func (s *Solver) Push() (err error) {
	if err = s.open(); err != nil {
		return err
	}
	if _, err = s.internalizeFormulas(); err != nil {
		return err
	}
	if err = s.encoder.Push(); err != nil {
		return err
	}
	s.scopes = append(s.scopes, solverScope{len(s.formulas), len(s.tracked), s.head})
	return nil
}
func (s *Solver) Pop(n uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	if n > uint32(len(s.scopes)) {
		n = uint32(len(s.scopes))
	}
	if err := s.encoder.Pop(n); err != nil {
		return err
	}
	for n > 0 {
		i := len(s.scopes) - 1
		f := s.scopes[i]
		s.head = f.head
		if err := releaseRoots(s.manager, &s.formulas, f.formulas); err != nil {
			return err
		}
		if err := releaseRoots(s.manager, &s.tracked, f.assumptions); err != nil {
			return err
		}
		s.scopes = s.scopes[:i]
		n--
	}
	return nil
}
func (s *Solver) newGoal(cores bool) (*tactic.Goal, error) {
	g, err := tactic.NewDefaultGoal(s.manager, true, cores)
	if err != nil {
		return nil, err
	}
	s.goals = append(s.goals, g)
	return g, nil
}
func (s *Solver) compile(g *tactic.Goal) (lbool.LBool, error) {
	if err := s.encoder.last.Prepare(s.manager); err != nil {
		return lbool.Undef, err
	}
	for i, owned := range s.goals {
		if owned == g {
			s.goals = append(s.goals[:i], s.goals[i+1:]...)
			break
		}
	}
	if s.lastGoal != nil {
		s.goals = append(s.goals, s.lastGoal)
	}
	s.lastGoal = g
	if err := s.drainGoals(); err != nil {
		return lbool.Undef, err
	}
	status, err := s.encoder.Compile(g)
	if err == nil && status == lbool.Undef && s.encoder.translation != nil {
		atoms, e := s.encoder.translation.InterpretedAtoms()
		if e != nil {
			return status, e
		}
		if len(atoms) > 0 {
			text, e := s.formatExpressions(atoms)
			if e != nil {
				return status, e
			}
			s.unknown = "interpreted atoms sent to SAT solver " + text
			if e = s.emitUnknown(); e != nil {
				return status, e
			}
		}
	}
	return status, err
}
func (s *Solver) internalizeFormulas() (status lbool.LBool, err error) {
	defer func() { err = errors.Join(err, s.drainGoals()) }()
	if int(s.head) == len(s.formulas) {
		return lbool.True, nil
	}
	g, err := s.newGoal(false)
	if err != nil {
		return lbool.Undef, err
	}
	for _, f := range s.formulas[s.head:] {
		if err = g.Assert(f); err != nil {
			return lbool.Undef, err
		}
	}
	status, err = s.compile(g)
	if err == nil && status != lbool.Undef {
		s.head = uint32(len(s.formulas))
	}
	return status, err
}
func (s *Solver) internalizeAssumptions(asms []ast.ASTHandle) (status lbool.LBool, deps map[ast.ASTHandle]sat.Literal, err error) {
	defer func() { err = errors.Join(err, s.drainGoals()) }()
	deps = make(map[ast.ASTHandle]sat.Literal)
	if len(asms) == 0 && len(s.tracked) == 0 {
		s.assumptions = s.assumptions[:0]
		return lbool.True, deps, nil
	}
	g, err := s.newGoal(true)
	if err != nil {
		return lbool.Undef, deps, err
	}
	for _, forms := range [][]ast.ASTHandle{asms, s.tracked} {
		for _, f := range forms {
			leaf, e := s.manager.DependencyLeaf(f)
			if e != nil {
				return lbool.Undef, deps, e
			}
			if e = g.AssertWithProof(f, ast.ASTHandle{}, leaf); e != nil {
				return lbool.Undef, deps, e
			}
		}
	}
	status, err = s.compile(g)
	if err != nil || status != lbool.True {
		return status, deps, err
	}
	bindings, err := s.encoder.translation.DependencyLiterals()
	if err != nil {
		return lbool.Undef, deps, err
	}
	for _, b := range bindings {
		deps[b.Expression] = b.Literal
	}
	s.assumptions = s.assumptions[:0]
	j := 0
	for group, forms := range [][]ast.ASTHandle{asms, s.tracked} {
		for i, f := range forms {
			if l, found := deps[f]; found {
				s.assumptions = append(s.assumptions, l)
				if group == 0 && len(s.weights) > 0 {
					// Only the explicit assumptions compact source m_weights;
					// automatic trackers have no corresponding weight entries.
					if i != j {
						if i >= len(s.weights) || j >= len(s.weights) {
							return lbool.Undef, deps, arena.ErrBounds
						}
						s.weights[j] = s.weights[i]
					}
				}
				if group == 0 {
					j++
				}
			}
		}
	}
	if ast.ASTSourceDebug && len(deps) != len(s.assumptions) {
		return lbool.Undef, deps, ast.ErrASTDomain
	}
	return status, deps, nil
}
func (s *Solver) isLiteral(h ast.ASTHandle) (bool, error) {
	negative, inner, err := s.manager.IsNot(h)
	if err != nil {
		return false, err
	}
	if negative {
		h = inner
	}
	kind, err := s.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	n, err := s.manager.GetNumArgs(h)
	if err != nil || n != 0 {
		return false, err
	}
	family, err := s.manager.GetFamilyID(h)
	return family == ast.NullFamilyID, err
}
func (s *Solver) CheckSat(asms []ast.ASTHandle) (status lbool.LBool, err error) {
	defer func() {
		s.retainFailure(err)
		err = errors.Join(err, s.drainChecks(), s.drainGoals(), s.drainModelWork())
		if err == nil {
			s.status = status
		}
	}()
	if err = s.open(); err != nil {
		return lbool.Undef, err
	}
	if err = s.core.PopToBaseLevel(); err != nil {
		return lbool.Undef, err
	}
	w := &checkWork{owner: s, original: make(map[ast.ASTHandle]ast.ASTHandle)}
	s.checks = append(s.checks, w)
	for _, f := range asms {
		literal, e := s.isLiteral(f)
		if e != nil {
			return lbool.Undef, e
		}
		a := f
		if !literal {
			bs, e := s.manager.MkBoolSort()
			if e != nil {
				return lbool.Undef, e
			}
			a, e = s.manager.MkFreshConst("s", bs)
			if e != nil {
				return lbool.Undef, e
			}
			if e = s.hold(&w.held, a); e != nil {
				return lbool.Undef, e
			}
			eq, e := s.manager.MkEq(a, f)
			if e != nil {
				return lbool.Undef, e
			}
			if e = s.Assert(eq); e != nil {
				return lbool.Undef, e
			}
		} else {
			if e = s.hold(&w.held, a); e != nil {
				return lbool.Undef, e
			}
		}
		w.assumptions = append(w.assumptions, a)
		w.original[a] = f
	}
	if err = s.emitVector("check_sat", 156, w.assumptions); err != nil {
		return lbool.Undef, err
	}
	if err = s.model.Set(nil); err != nil {
		return lbool.Undef, err
	}
	s.modelPending = false
	status, err = s.internalizeFormulas()
	if err != nil || status != lbool.True {
		return status, err
	}
	status, deps, err := s.internalizeAssumptions(w.assumptions)
	if err != nil || status != lbool.True {
		return status, err
	}
	dimacs, err := s.prepareDIMACS()
	if err != nil {
		return lbool.Undef, err
	}
	status, err = s.core.Check(s.assumptions)
	err = errors.Join(err, s.finishDIMACS(dimacs, status == lbool.Undef))
	if err != nil {
		return status, err
	}
	if status == lbool.False && len(s.assumptions) > 0 {
		if err = s.extractCore(deps, w.original); err != nil {
			return status, err
		}
	}
	if status == lbool.True && len(asms) > 0 {
		values, e := s.core.AppendModel(nil)
		if e != nil {
			return status, e
		}
		for f, l := range deps {
			value, e := values.Read(l.Variable())
			if e != nil {
				return status, e
			}
			if l.Sign() {
				value = value.Negated()
			}
			if value != lbool.True {
				if e = s.badAssumption(f); e != nil {
					return status, e
				}
				return status, z3exception.NewDefaultException("bad state")
			}
		}
	}
	return status, nil
}
func (s *Solver) extractCore(deps map[ast.ASTHandle]sat.Literal, originals map[ast.ASTHandle]ast.ASTHandle) error {
	if err := s.emitCore(deps, originals); err != nil {
		return err
	}
	inverse := make(map[uint32]ast.ASTHandle)
	for f, l := range deps {
		inverse[l.Index()] = f
	}
	if err := releaseRoots(s.manager, &s.unsatCore, 0); err != nil {
		return err
	}
	core, err := s.core.BorrowCore()
	if err != nil {
		return err
	}
	n, err := core.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		l, e := core.Read(i)
		if e != nil {
			return e
		}
		f, found := inverse[l.Index()]
		if !found {
			return ast.ErrASTDomain
		}
		if original, found := originals[f]; found {
			f = original
		}
		if e = s.hold(&s.unsatCore, f); e != nil {
			return e
		}
	}
	return nil
}
func (s *Solver) UnsatCore() ([]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), s.unsatCore...), nil
}
