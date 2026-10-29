package solver

// Source: src/solver/tactic2solver.h; src/solver/tactic2solver.cpp.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

type TacticSolverConfig struct {
	Params           *uparams.ParamsContext
	EvaluatorFactory ConsequenceEvaluatorFactory
	TargetConfig     func(*ast.ASTManager) (TacticSolverConfig, error)
	Trace            *diagnostic.TraceTags
}

type TacticSolver struct {
	self                                       *TacticSolver
	manager                                    *ast.ASTManager
	config                                     TacticSolverConfig
	tactic                                     tactic.Tactic
	named                                      *NamedAssertions
	result                                     *SimpleCheckSatResult
	assertions                                 []ast.ASTRef
	consequences                               []ast.ASTRef
	scopes                                     []uint32
	params                                     uparams.ParamsRef
	logic                                      usymbol.Symbol
	stats                                      statistics.Statistics
	references                                 uint32
	produceProofs, produceModels, produceCores bool
	closed                                     bool
}

func NewTacticSolver(m *ast.ASTManager, t tactic.Tactic, p *uparams.ParamsRef, proofs, models, cores bool, logic string, config TacticSolverConfig) (*TacticSolver, error) {
	if m == nil || config.Params == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := config.Params.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	value := config.Params.EmptyValue()
	if p != nil {
		if err := config.Params.Copy(&value, p); err != nil {
			return nil, err
		}
	}
	logicSymbol := usymbol.Symbol{}
	var err error
	if logic != "" {
		logicSymbol, err = m.Symbols.Intern(logic)
		if err != nil {
			_ = config.Params.Del(&value)
			return nil, err
		}
	}
	s := &TacticSolver{manager: m, config: config, tactic: t, params: value, logic: logicSymbol, produceProofs: proofs, produceModels: models, produceCores: cores}
	s.self = s
	if t != nil {
		if err = t.IncRef(); err != nil {
			_ = config.Params.Del(&s.params)
			return nil, err
		}
	}
	s.named, err = NewNamedAssertions(m, s)
	if err != nil {
		if t != nil {
			_ = t.DecRef()
		}
		_ = config.Params.Del(&s.params)
		return nil, err
	}
	return s, nil
}

func (s *TacticSolver) open() error {
	if s == nil || s.self != s {
		return ast.ErrASTHandle
	}
	if s.closed {
		return arena.ErrClosed
	}
	return s.config.Params.Check(&s.params)
}

func (s *TacticSolver) IncRef() error {
	if err := s.open(); err != nil {
		return err
	}
	s.references++
	return nil
}
func (s *TacticSolver) DecRef() error {
	if err := s.open(); err != nil {
		return err
	}
	if s.references == 0 {
		return arena.ErrBounds
	}
	s.references--
	if s.references == 0 {
		return s.Close()
	}
	return nil
}

func (s *TacticSolver) clearResult() error {
	if s.result == nil {
		return nil
	}
	if err := s.result.Close(); err != nil {
		return err
	}
	s.result = nil
	return nil
}

func (s *TacticSolver) Status() (lbool.LBool, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	if s.result == nil {
		return lbool.Undef, nil
	}
	return s.result.Status()
}
func (s *TacticSolver) SetStatus(v lbool.LBool) error {
	if err := s.open(); err != nil {
		return err
	}
	if s.result == nil {
		// A combined solver can publish a command-context exception status
		// before this nonselected tactic solver has executed. Native stores
		// that status in the combined check_sat_result; there is no tactic
		// result to update on this branch.
		return nil
	}
	return s.result.SetStatus(v)
}
func (s *TacticSolver) CollectStatistics(dst *statistics.Statistics) error {
	if err := s.open(); err != nil {
		return err
	}
	if dst == nil {
		return arena.ErrInvalidSlot
	}
	dst.Copy(s.stats)
	return nil
}
func (s *TacticSolver) UnsatCore() ([]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if s.result == nil {
		return []ast.ASTHandle{}, nil
	}
	return s.result.UnsatCore()
}

func (s *TacticSolver) clearConsequences() error {
	for len(s.consequences) != 0 {
		i := len(s.consequences) - 1
		if err := s.manager.DelRef(&s.consequences[i]); err != nil {
			return err
		}
		s.consequences = s.consequences[:i]
	}
	return nil
}

func (s *TacticSolver) RetainConsequence(value ast.ASTHandle) (ast.ASTHandle, error) {
	ref, err := s.manager.NewRef(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	s.consequences = append(s.consequences, ref)
	return s.manager.RefGet(ref)
}
func (s *TacticSolver) AcquireModel() (*model.ModelCore, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if s.result == nil {
		return nil, nil
	}
	return s.result.AcquireModel()
}
func (s *TacticSolver) Proof() (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if s.result == nil {
		return ast.ASTHandle{}, nil
	}
	return s.result.Proof()
}
func (s *TacticSolver) ReasonUnknown() (string, error) {
	if err := s.open(); err != nil {
		return "", err
	}
	if s.result == nil {
		return "unknown", nil
	}
	return s.result.ReasonUnknown()
}
func (s *TacticSolver) SetReasonUnknown(v string) error {
	if err := s.open(); err != nil {
		return err
	}
	if s.result == nil {
		return nil
	}
	return s.result.SetReasonUnknown(v)
}
func (s *TacticSolver) Labels() ([]usymbol.Symbol, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if s.result == nil {
		return []usymbol.Symbol{}, nil
	}
	return s.result.Labels()
}
func (s *TacticSolver) Manager() (*ast.ASTManager, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return s.manager, nil
}

func (s *TacticSolver) UpdateParams(p *uparams.ParamsRef) error {
	if err := s.open(); err != nil {
		return err
	}
	if p == nil {
		empty := s.config.Params.EmptyValue()
		p = &empty
	}
	return s.config.Params.Copy(&s.params, p)
}
func (s *TacticSolver) Params() (*uparams.ParamsRef, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return &s.params, nil
}
func (s *TacticSolver) CollectParamDescrs(d *uparams.ParamDescrs) error {
	if err := s.open(); err != nil {
		return err
	}
	if s.tactic == nil {
		return nil
	}
	return s.tactic.CollectParamDescrs(d)
}
func (s *TacticSolver) SetProduceModels(v bool) error {
	if err := s.open(); err != nil {
		return err
	}
	s.produceModels = v
	return nil
}

func (s *TacticSolver) AssertFormula(h ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	ref, err := s.manager.NewRef(h)
	if err != nil {
		return err
	}
	s.assertions = append(s.assertions, ref)
	return s.clearResult()
}
func (s *TacticSolver) Assert(h ast.ASTHandle) error { return s.AssertFormula(h) }
func (s *TacticSolver) AssertTracked(h, tag ast.ASTHandle) error {
	return s.named.AssertTracked(h, tag)
}

func (s *TacticSolver) PushCore() error {
	s.scopes = append(s.scopes, uint32(len(s.assertions)))
	return s.clearResult()
}
func (s *TacticSolver) Push() error { return s.named.Push() }
func (s *TacticSolver) restoreAssertions(size uint32) error {
	for uint32(len(s.assertions)) > size {
		i := len(s.assertions) - 1
		if err := s.manager.DelRef(&s.assertions[i]); err != nil {
			return err
		}
		s.assertions = s.assertions[:i]
	}
	return nil
}
func (s *TacticSolver) PopCore(n uint32) error {
	if n > uint32(len(s.scopes)) {
		return arena.ErrBounds
	}
	level := uint32(len(s.scopes)) - n
	if err := s.restoreAssertions(s.scopes[level]); err != nil {
		return err
	}
	s.scopes = s.scopes[:level]
	return s.clearResult()
}
func (s *TacticSolver) Pop(n uint32) error          { return s.named.Pop(n) }
func (s *TacticSolver) ScopeLevel() (uint32, error) { return s.named.ScopeLevel() }

func (s *TacticSolver) assertionHandles() ([]ast.ASTHandle, error) {
	values := make([]ast.ASTHandle, len(s.assertions))
	for i := range s.assertions {
		v, err := s.manager.RefGet(s.assertions[i])
		if err != nil {
			return nil, err
		}
		values[i] = v
	}
	return values, nil
}
func (s *TacticSolver) NumAssertions() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return uint32(len(s.assertions)), nil
}
func (s *TacticSolver) Assertion(i uint32) (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if uint64(i) >= uint64(len(s.assertions)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return s.manager.RefGet(s.assertions[i])
}
func (s *TacticSolver) NumAssumptions() (uint32, error)              { return s.named.NumAssumptions() }
func (s *TacticSolver) Assumption(i uint32) (ast.ASTHandle, error)   { return s.named.Assumption(i) }
func (s *TacticSolver) DisplayCore(w io.Writer) error                { return DefaultDisplay(s, w, nil) }
func (s *TacticSolver) Display(w io.Writer, a []ast.ASTHandle) error { return DefaultDisplay(s, w, a) }

func (s *TacticSolver) replaceResult(next *SimpleCheckSatResult) error {
	if err := s.clearResult(); err != nil {
		return err
	}
	s.result = next
	return nil
}

func (s *TacticSolver) traceException(cause error) {
	tags := s.config.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("tactic2solver", "check_sat_core", "src/solver/tactic2solver.cpp", 117, func() bool { return true }, func(w io.Writer) error {
		_, err := io.WriteString(w, "exception: "+cause.Error()+"\n")
		return err
	})
}

func (s *TacticSolver) CheckSatCore(assumptions []ast.ASTHandle) (status lbool.LBool, err error) {
	if err = s.open(); err != nil {
		return lbool.Undef, err
	}
	if s.tactic == nil {
		return lbool.False, nil
	}
	result, err := NewSimpleCheckSatResult(s.manager)
	if err != nil {
		return lbool.Undef, err
	}
	if err = s.replaceResult(result); err != nil {
		_ = result.Close()
		return lbool.Undef, err
	}
	if err = s.tactic.Cleanup(); err != nil {
		return lbool.Undef, err
	}
	if err = s.tactic.SetLogic(s.logic); err != nil {
		return lbool.Undef, err
	}
	if err = s.tactic.UpdateParams(&s.params); err != nil {
		return lbool.Undef, err
	}
	goal, err := tactic.NewGoal(s.manager, s.produceProofs, s.produceModels, s.produceCores)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { err = errors.Join(err, goal.Close()) }()
	assertions, err := s.assertionHandles()
	if err != nil {
		return lbool.Undef, err
	}
	for _, h := range assertions {
		if err = goal.Assert(h); err != nil {
			return lbool.Undef, err
		}
	}
	for _, h := range assumptions {
		var proof ast.ASTHandle
		if s.produceProofs {
			if proof, err = s.manager.MkProofAsserted(h); err != nil {
				return lbool.Undef, err
			}
		}
		var dep ast.ASTDependency
		if s.produceCores {
			if dep, err = s.manager.DependencyLeaf(h); err != nil {
				return lbool.Undef, err
			}
		}
		if err = goal.AssertWithProof(h, proof, dep); err != nil {
			return lbool.Undef, err
		}
	}
	md := &tactic.ModelRef{}
	defer func() { err = errors.Join(err, md.Close()) }()
	labels := &tactic.ModelLabels{Budget: s.manager.Budget()}
	defer labels.Close()
	proofRef, err := s.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { err = errors.Join(err, s.manager.DelRef(&proofRef)) }()
	core := &tactic.DependencyRef{}
	defer func() { err = errors.Join(err, core.Close()) }()
	reason := "unknown"
	status, reason, err = tactic.CheckSat(s.tactic, goal, md, labels, proofRef, core)
	if err != nil {
		s.traceException(err)
		// A retained Go owner is not a recoverable source tactic exception.
		// Preserve its cleanup receipt instead of reducing it to a string.
		var owned memory.ConstructionFailure
		if errors.As(err, &owned) {
			return lbool.Undef, err
		}
		var fatal z3exception.Z3Error
		if errors.As(err, &fatal) {
			return lbool.Undef, err
		}
		status = lbool.Undef
		reason = err.Error()
		err = nil
	}
	if err = result.SetStatus(status); err != nil {
		return lbool.Undef, err
	}
	if status == lbool.Undef && reason != "" {
		if err = result.SetReasonUnknown(reason); err != nil {
			return lbool.Undef, err
		}
	}
	if value, e := md.Get(); e != nil {
		return lbool.Undef, e
	} else if e = result.SetModel(value); e != nil {
		return lbool.Undef, e
	}
	if value, e := s.manager.RefGet(proofRef); e != nil {
		return lbool.Undef, e
	} else if e = result.SetProof(value); e != nil {
		return lbool.Undef, e
	}
	if s.produceCores {
		values, e := s.manager.DependencyLinearize(core.Get(), nil)
		if e != nil {
			return lbool.Undef, e
		}
		if e = result.SetUnsatCore(values); e != nil {
			return lbool.Undef, e
		}
	}
	if err = result.SetLabels(labels.Values); err != nil {
		return lbool.Undef, err
	}
	for key, value := range s.tactic.CollectStatistics() {
		result.stats.UpdateDouble(statistics.Key(key), value)
		s.stats.UpdateDouble(statistics.Key(key), value)
	}
	if err = s.tactic.Cleanup(); err != nil {
		return lbool.Undef, err
	}
	return status, nil
}

func (s *TacticSolver) CheckSat(a []ast.ASTHandle) (lbool.LBool, error) { return s.named.CheckSat(a) }
func (s *TacticSolver) GetConsequences(a, v []ast.ASTHandle) (lbool.LBool, []ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, nil, err
	}
	if s.config.EvaluatorFactory == nil {
		return lbool.Undef, nil, ast.ErrASTHandle
	}
	if err := s.clearConsequences(); err != nil {
		return lbool.Undef, nil, err
	}
	return s.named.WithAssumptions(a, func(combined []ast.ASTHandle) (lbool.LBool, []ast.ASTHandle, error) {
		return DefaultGetConsequences(s, combined, v, s.config.EvaluatorFactory)
	})
}
func (s *TacticSolver) FindMutexes(v []ast.ASTHandle) (lbool.LBool, [][]ast.ASTHandle, error) {
	return DefaultFindMutexes(v)
}
func (s *TacticSolver) PreferredSAT(a []ast.ASTHandle) (lbool.LBool, [][]ast.ASTHandle, error) {
	return DefaultPreferredSAT(s)
}
func (s *TacticSolver) SetProgressCallback(ProgressCallback) error { return s.open() }

func (s *TacticSolver) Translate(m *ast.ASTManager, p *uparams.ParamsRef) (Solver, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if len(s.scopes) != 0 {
		return nil, z3exception.NewDefaultException("translation of contexts is only supported at base level")
	}
	if s.tactic == nil {
		return nil, ast.ErrASTHandle
	}
	translated, err := s.tactic.Translate(m)
	if err != nil {
		return nil, err
	}
	logic := ""
	if s.logic != (usymbol.Symbol{}) {
		logic, err = s.manager.Symbols.String(s.logic)
		if err != nil {
			return nil, err
		}
	}
	config := s.config
	if check := config.Params.CheckServices(m.Symbols, m.NumeralService()); check != nil {
		if config.TargetConfig == nil {
			return nil, check
		}
		config, err = config.TargetConfig(m)
		if err != nil {
			return nil, err
		}
	}
	next, err := NewTacticSolver(m, translated, p, s.produceProofs, s.produceModels, s.produceCores, logic, config)
	if err != nil {
		return nil, err
	}
	values, err := s.assertionHandles()
	if err != nil {
		_ = next.Close()
		return nil, err
	}
	if m == s.manager {
		for _, h := range values {
			if err = next.Assert(h); err != nil {
				_ = next.Close()
				return nil, err
			}
		}
		return next, nil
	}
	tr, err := ast.NewASTTranslation(s.manager, m, false)
	if err != nil {
		_ = next.Close()
		return nil, err
	}
	defer tr.Close()
	for _, h := range values {
		v, e := tr.Translate(h)
		if e != nil {
			_ = next.Close()
			return nil, e
		}
		if e = next.Assert(v); e != nil {
			_ = next.Close()
			return nil, e
		}
	}
	return next, nil
}

func (s *TacticSolver) Close() (err error) {
	if s == nil || s.self != s {
		return ast.ErrASTHandle
	}
	if s.closed {
		return nil
	}
	if err = s.clearConsequences(); err != nil {
		return err
	}
	if err = s.clearResult(); err != nil {
		return err
	}
	if err = s.named.Close(); err != nil {
		return err
	}
	if err = s.restoreAssertions(0); err != nil {
		return err
	}
	if err = s.config.Params.Del(&s.params); err != nil {
		return err
	}
	if s.tactic != nil {
		if err = s.tactic.DecRef(); err != nil {
			return err
		}
		s.tactic = nil
	}
	s.closed = true
	return nil
}

type tacticSolverFactory struct {
	tactic tactic.Tactic
	config TacticSolverConfig
	closed bool
}

func NewTacticSolverFactory(t tactic.Tactic, config TacticSolverConfig) (Factory, error) {
	if t == nil {
		return nil, ast.ErrASTHandle
	}
	if err := t.IncRef(); err != nil {
		return nil, err
	}
	return &tacticSolverFactory{tactic: t, config: config}, nil
}
func (f *tacticSolverFactory) New(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, logic string) (Solver, error) {
	if f == nil || f.closed {
		return nil, arena.ErrClosed
	}
	return NewTacticSolver(m, f.tactic, p, proofs, models, cores, logic, f.config)
}
func (f *tacticSolverFactory) Close() error {
	if f == nil || f.closed {
		return nil
	}
	if err := f.tactic.DecRef(); err != nil {
		return err
	}
	f.tactic, f.closed = nil, true
	return nil
}

type tacticFactorySolverFactory struct {
	factory tactic.TacticFactory
	config  TacticSolverConfig
	closed  bool
}

func NewTacticFactorySolverFactory(f tactic.TacticFactory, config TacticSolverConfig) (Factory, error) {
	if f == nil {
		return nil, ast.ErrASTHandle
	}
	return &tacticFactorySolverFactory{factory: f, config: config}, nil
}
func (f *tacticFactorySolverFactory) New(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, logic string) (Solver, error) {
	if f == nil || f.closed {
		return nil, arena.ErrClosed
	}
	t, err := f.factory.New(m, p)
	if err != nil {
		return nil, err
	}
	return NewTacticSolver(m, t, p, proofs, models, cores, logic, f.config)
}
func (f *tacticFactorySolverFactory) Close() error {
	if f == nil || f.closed {
		return nil
	}
	if err := f.factory.Close(); err != nil {
		return err
	}
	f.factory, f.closed = nil, true
	return nil
}
