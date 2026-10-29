package smtcore

// Source: src/smt/smt_solver.h; src/smt/smt_solver.cpp retained kernel-backed
// solver facade. Model, proof-result, consequence, mutex, core-extension, and
// cross-manager kernel-copy operations remain with their kernel prerequisites.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/cliques"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

type SMTSolverConfig struct {
	Params         *uparams.ParamsContext
	Globals        *uparams.GlobalParamsService
	Quantifiers    *SMTQuantifierRegistry
	ModelEvaluator model.ModelEvaluatorConfig
	TargetConfig   func(*ast.ASTManager) (SMTSolverConfig, error)
}

type smtTrackedAssertion struct {
	tag     ast.ASTHandle
	formula ast.ASTRef
}

type smtSolverObjective struct {
	term     ast.ASTRef
	theory   smtArithmeticObjectiveTheory
	variable SMTTheoryVar
}

// SMTSolver exposes the retained SMT kernel through solver.Solver. The result
// object is replaced on each check, while kernel statistics remain cumulative.
type SMTSolver struct {
	self          *SMTSolver
	manager       *ast.ASTManager
	config        SMTSolverConfig
	kernel        *SMTKernel
	named         *solver.NamedAssertions
	result        *solver.SimpleCheckSatResult
	params        uparams.ParamsRef
	logic         usymbol.Symbol
	tracked       []smtTrackedAssertion
	trackedScopes []uint32
	consequences  []ast.ASTRef
	objectives    []smtSolverObjective
	callback      solver.ProgressCallback
	references    uint32
	produceModels bool
	ownedCatalog  *uparams.GlobalParamsCatalog
	ownedGlobals  *uparams.GlobalParamsService
	closed        bool
}

var _ solver.Solver = (*SMTSolver)(nil)
var _ solver.CandidateModelProvider = (*SMTSolver)(nil)

func smtSolverGlobals(m *ast.ASTManager, config SMTSolverConfig) (*uparams.GlobalParamsService, *uparams.GlobalParamsCatalog, error) {
	if config.Globals != nil {
		if config.Globals.Context != config.Params {
			return nil, nil, uparams.ErrParamsOwner
		}
		return config.Globals, nil, config.Globals.CheckOpen()
	}
	if globals := config.ModelEvaluator.Globals; globals != nil && globals.Context == config.Params {
		return globals, nil, globals.CheckOpen()
	}
	if globals := uparams.BorrowProcessGlobalParamsService(); globals != nil && globals.Context == config.Params {
		return globals, nil, globals.CheckOpen()
	}
	catalog, err := uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: m.Budget()})
	if err != nil {
		return nil, nil, err
	}
	globals, err := uparams.NewGlobalParamsService(m.Symbols, config.Params, uparams.GlobalParamsConfig{Budget: m.Budget(), Catalog: catalog})
	if err != nil {
		return nil, catalog, err
	}
	if err = globals.InitializeSource(); err != nil {
		return globals, catalog, err
	}
	return globals, catalog, nil
}

func NewSMTSolver(m *ast.ASTManager, p *uparams.ParamsRef, logic string, config SMTSolverConfig) (*SMTSolver, error) {
	return newSMTSolver(m, p, logic, config, nil)
}

// Tactics pass a persistent fparams snapshot to their temporary context.
func newSMTSolver(m *ast.ASTManager, p *uparams.ParamsRef, logic string, config SMTSolverConfig, sourceParams *SMTParams) (*SMTSolver, error) {
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
	globals, ownedCatalog, err := smtSolverGlobals(m, config)
	if err != nil {
		if globals != nil {
			err = errors.Join(err, globals.Close())
		}
		if ownedCatalog != nil {
			err = errors.Join(err, ownedCatalog.Close())
		}
		return nil, errors.Join(err, config.Params.Del(&value))
	}
	ownedGlobals := globals
	if ownedCatalog == nil {
		ownedGlobals = nil
	}
	cleanupServices := func(cause error) error {
		if ownedGlobals != nil {
			cause = errors.Join(cause, ownedGlobals.Close())
		}
		if ownedCatalog != nil {
			cause = errors.Join(cause, ownedCatalog.Close())
		}
		return cause
	}
	logicSymbol := usymbol.Symbol{}
	if logic != "" {
		logicSymbol, err = m.Symbols.Intern(logic)
		if err != nil {
			return nil, cleanupServices(errors.Join(err, config.Params.Del(&value)))
		}
	}
	kernel, err := NewSMTKernel(m, config.Quantifiers)
	if err != nil {
		return nil, cleanupServices(errors.Join(err, config.Params.Del(&value)))
	}
	if err = kernel.configurePreprocessing(config.Params, globals, &value, sourceParams); err != nil {
		return nil, cleanupServices(errors.Join(err, kernel.Close(), config.Params.Del(&value)))
	}
	if err = smtInstallRetainedSetup(kernel, m, logic); err != nil {
		return nil, cleanupServices(errors.Join(err, kernel.Close(), config.Params.Del(&value)))
	}
	s := &SMTSolver{manager: m, config: config, kernel: kernel, params: value, logic: logicSymbol, ownedCatalog: ownedCatalog, ownedGlobals: ownedGlobals}
	s.self = s
	s.named, err = solver.NewNamedAssertions(m, s)
	if err != nil {
		return nil, cleanupServices(errors.Join(err, kernel.Close(), config.Params.Del(&value)))
	}
	if manager := kernel.internalizer.quantifierManager; manager != nil {
		if err = manager.SetAuxiliarySolverFactory(s.newAuxiliarySolver); err != nil {
			return nil, cleanupServices(errors.Join(err, s.named.Close(), kernel.Close(), config.Params.Del(&value)))
		}
	}
	return s, nil
}

func (s *SMTSolver) newAuxiliarySolver() (auxiliary *SMTSolver, resultErr error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	config := s.config
	config.Quantifiers = nil
	config.Globals = s.kernel.paramGlobals
	config.ModelEvaluator = s.kernel.modelConfig
	// model_checker::init_aux_context snapshots fparams once. Empty logic and
	// these two overrides are independent of the parent context's setup.
	snapshot, err := s.kernel.smtParams.Clone()
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, snapshot.Close())
		if resultErr != nil && auxiliary != nil {
			resultErr = errors.Join(resultErr, auxiliary.Close())
			auxiliary = nil
		}
	}()
	snapshot.state.RelevancyLvl = 0
	snapshot.state.CaseSplitStrategy = 0 // CS_ACTIVITY
	auxiliary, err = newSMTSolver(s.manager, nil, "", config, snapshot)
	if err != nil {
		return nil, err
	}
	if err = auxiliary.kernel.copyArithmeticPlugin(s.kernel); err != nil {
		return auxiliary, err
	}
	if err = auxiliary.kernel.copyBitVectorPlugin(s.kernel); err != nil {
		return auxiliary, err
	}
	if err = auxiliary.kernel.copyFloatingPointPlugin(s.kernel); err != nil {
		return auxiliary, err
	}
	if err = auxiliary.kernel.copyArrayPlugin(s.kernel); err != nil {
		return auxiliary, err
	}
	if err = auxiliary.kernel.copySequencePlugin(s.kernel); err != nil {
		return auxiliary, err
	}
	if err = auxiliary.kernel.copyDatatypePlugin(s.kernel); err != nil {
		return auxiliary, err
	}
	if err = auxiliary.kernel.copyPseudoBooleanPlugin(s.kernel); err != nil {
		return auxiliary, err
	}
	if err = auxiliary.SetProduceModels(true); err != nil {
		return auxiliary, err
	}
	return auxiliary, nil
}

func (s *SMTSolver) open() error {
	if s == nil || s.self != s {
		return ast.ErrASTHandle
	}
	if s.closed {
		return arena.ErrClosed
	}
	return s.config.Params.Check(&s.params)
}

func (s *SMTSolver) IncRef() error {
	if err := s.open(); err != nil {
		return err
	}
	s.references++
	return nil
}

func (s *SMTSolver) DecRef() error {
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

func (s *SMTSolver) clearResult() error {
	if s.result == nil {
		return nil
	}
	err := s.result.Close()
	if err == nil {
		s.result = nil
	}
	return err
}

func (s *SMTSolver) replaceResult(next *solver.SimpleCheckSatResult) error {
	if err := s.clearResult(); err != nil {
		return err
	}
	s.result = next
	return nil
}

func (s *SMTSolver) Status() (lbool.LBool, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	if s.result == nil {
		return lbool.Undef, nil
	}
	return s.result.Status()
}

func (s *SMTSolver) SetStatus(v lbool.LBool) error {
	if err := s.open(); err != nil {
		return err
	}
	if s.result == nil {
		return ast.ErrASTHandle
	}
	return s.result.SetStatus(v)
}

func collectSMTKernelStatistics(dst *statistics.Statistics, value SMTContextStatistics) {
	dst.UpdateUint("conflicts", value.NumConflicts)
	dst.UpdateUint("decisions", value.NumDecisions)
	dst.UpdateUint("propagations", value.NumPropagations+value.NumBinPropagations)
	dst.UpdateUint("binary propagations", value.NumBinPropagations)
	dst.UpdateUint("restarts", value.NumRestarts)
	dst.UpdateUint("final checks", value.NumFinalChecks)
	dst.UpdateUint("added eqs", value.NumAddEq)
	dst.UpdateUint("mk clause", value.NumMkClause)
	dst.UpdateUint("del clause", value.NumDelClause)
	dst.UpdateUint("dyn ack", value.NumDynAck)
	dst.UpdateUint("interface eqs", value.NumInterfaceEqs)
	dst.UpdateUint("max generation", value.MaxGeneration)
	dst.UpdateUint("minimized lits", value.NumMinimizedLits)
	dst.UpdateUint("num checks", value.NumChecks)
	dst.UpdateUint("mk bool var", value.NumMkBoolVar)
}

func (s *SMTSolver) CollectStatistics(dst *statistics.Statistics) error {
	if err := s.open(); err != nil {
		return err
	}
	if dst == nil {
		return arena.ErrInvalidSlot
	}
	collectSMTKernelStatistics(dst, s.kernel.Statistics())
	for _, theory := range s.kernel.internalizer.theories {
		if collector, ok := theory.(interface {
			CollectStatistics(*statistics.Statistics) error
		}); ok {
			if err := collector.CollectStatistics(dst); err != nil {
				return err
			}
		}
	}
	return nil
}

// ResetStatistics follows smt::kernel::reset_statistics. The source kernel
// currently preserves its cumulative context counters, so this is an open-state
// check until that native operation gains additional behavior.
func (s *SMTSolver) ResetStatistics() error {
	if err := s.open(); err != nil {
		return err
	}
	return s.kernel.ResetStatistics()
}

func (s *SMTSolver) UnsatCore() ([]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if s.result == nil {
		return []ast.ASTHandle{}, nil
	}
	return s.result.UnsatCore()
}

func (s *SMTSolver) clearConsequences() error {
	for len(s.consequences) != 0 {
		i := len(s.consequences) - 1
		if err := s.manager.DelRef(&s.consequences[i]); err != nil {
			return err
		}
		s.consequences = s.consequences[:i]
	}
	return nil
}

func (s *SMTSolver) RetainConsequence(value ast.ASTHandle) (ast.ASTHandle, error) {
	ref, err := s.manager.NewRef(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	s.consequences = append(s.consequences, ref)
	return s.manager.RefGet(ref)
}

func (s *SMTSolver) AcquireModel() (*model.ModelCore, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if s.result == nil {
		return nil, nil
	}
	return s.result.AcquireModel()
}

func (s *SMTSolver) Proof() (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if s.result == nil {
		return ast.ASTHandle{}, nil
	}
	return s.result.Proof()
}

func (s *SMTSolver) ReasonUnknown() (string, error) {
	if err := s.open(); err != nil {
		return "", err
	}
	if s.result == nil {
		return s.kernel.ReasonUnknown()
	}
	return s.result.ReasonUnknown()
}

func (s *SMTSolver) SetReasonUnknown(v string) error {
	if err := s.open(); err != nil {
		return err
	}
	if err := s.kernel.SetReasonUnknown(v); err != nil {
		return err
	}
	if s.result == nil {
		return nil
	}
	return s.result.SetReasonUnknown(v)
}

func (s *SMTSolver) Labels() ([]usymbol.Symbol, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if s.result == nil {
		return []usymbol.Symbol{}, nil
	}
	return s.result.Labels()
}

func (s *SMTSolver) Manager() (*ast.ASTManager, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return s.manager, nil
}

func (s *SMTSolver) UpdateParams(p *uparams.ParamsRef) error {
	if err := s.open(); err != nil {
		return err
	}
	if p == nil {
		empty := s.config.Params.EmptyValue()
		p = &empty
	}
	if err := s.config.Params.Copy(&s.params, p); err != nil {
		return err
	}
	return s.kernel.UpdateParams(&s.params)
}

func (s *SMTSolver) Params() (*uparams.ParamsRef, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return &s.params, nil
}

func (s *SMTSolver) CollectParamDescrs(d *uparams.ParamDescrs) error {
	if err := s.open(); err != nil {
		return err
	}
	if d == nil {
		return uparams.ErrParamDescrsPrecondition
	}
	return catalogCollectSmtParamsHelper(d)
}

func (s *SMTSolver) SetProduceModels(value bool) error {
	if err := s.open(); err != nil {
		return err
	}
	s.produceModels = value
	config := s.config.ModelEvaluator
	if config.Context == nil {
		// Keep default evaluator services local to this kernel, including
		// when a solver is translated for an individual tactic application.
		config = s.kernel.modelConfig
	}
	config.Params = &s.params
	return s.kernel.ConfigureModelGeneration(value, config)
}

func (s *SMTSolver) AssertFormula(formula ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	if err := s.kernel.Assert(formula, ast.ASTHandle{}); err != nil {
		return err
	}
	return s.clearResult()
}

func (s *SMTSolver) Assert(formula ast.ASTHandle) error { return s.AssertFormula(formula) }

func (s *SMTSolver) AssertTracked(formula, tag ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	if tag.IsNull() {
		return s.AssertFormula(formula)
	}
	for _, assertion := range s.tracked {
		if assertion.tag == tag {
			return z3exception.NewDefaultException("named assertion defined twice")
		}
	}
	ref, err := s.manager.NewRef(formula)
	if err != nil {
		return err
	}
	if err = s.named.AssertTracked(formula, tag); err != nil {
		return errors.Join(err, s.manager.DelRef(&ref))
	}
	s.tracked = append(s.tracked, smtTrackedAssertion{tag: tag, formula: ref})
	return nil
}

func (s *SMTSolver) PushCore() error {
	if err := s.open(); err != nil {
		return err
	}
	if err := s.kernel.Push(); err != nil {
		return err
	}
	s.trackedScopes = append(s.trackedScopes, uint32(len(s.tracked)))
	return s.clearResult()
}

func (s *SMTSolver) Push() error { return s.named.Push() }

func (s *SMTSolver) restoreTracked(size uint32) error {
	for uint32(len(s.tracked)) > size {
		index := len(s.tracked) - 1
		if err := s.manager.DelRef(&s.tracked[index].formula); err != nil {
			return err
		}
		s.tracked = s.tracked[:index]
	}
	return nil
}

func (s *SMTSolver) PopCore(count uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	if count > uint32(len(s.trackedScopes)) {
		return arena.ErrBounds
	}
	level := uint32(len(s.trackedScopes)) - count
	if err := s.kernel.Pop(count); err != nil {
		return err
	}
	if err := s.restoreTracked(s.trackedScopes[level]); err != nil {
		return err
	}
	s.trackedScopes = s.trackedScopes[:level]
	return s.clearResult()
}

func (s *SMTSolver) Pop(count uint32) error { return s.named.Pop(count) }

func (s *SMTSolver) ScopeLevel() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return s.named.ScopeLevel()
}

func (s *SMTSolver) CheckSatCore(assumptions []ast.ASTHandle) (lbool.LBool, error) {
	return s.checkSatCore(assumptions, false)
}

// The fresh tactic context uses setup_and_check without external assumptions.
// Both entry paths publish results through the same owning result object.
func (s *SMTSolver) checkSatCore(assumptions []ast.ASTHandle, setup bool) (lbool.LBool, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	result, err := solver.NewSimpleCheckSatResult(s.manager)
	if err != nil {
		return lbool.Undef, err
	}
	if err = s.replaceResult(result); err != nil {
		_ = result.Close()
		return lbool.Undef, err
	}
	var status lbool.LBool
	if setup {
		status, err = s.kernel.SetupAndCheck()
	} else {
		status, err = s.kernel.CheckAssumptions(assumptions)
	}
	if err != nil {
		return lbool.Undef, err
	}
	if err = result.SetStatus(status); err != nil {
		return lbool.Undef, err
	}
	if status == lbool.True && s.produceModels {
		if err = result.SetModel(s.kernel.GeneratedModel()); err != nil {
			return lbool.Undef, err
		}
	}
	if status == lbool.Undef {
		reason, reasonErr := s.kernel.ReasonUnknown()
		if reasonErr != nil {
			return lbool.Undef, reasonErr
		}
		if err = result.SetReasonUnknown(reason); err != nil {
			return lbool.Undef, err
		}
	}
	labels, labelsErr := s.kernel.Labels()
	if labelsErr != nil {
		return lbool.Undef, labelsErr
	}
	if err = result.SetLabels(labels); err != nil {
		return lbool.Undef, err
	}
	if status == lbool.False {
		core, coreErr := s.extendUnsatCore(s.kernel.UnsatCore())
		if coreErr != nil {
			return lbool.Undef, coreErr
		}
		if err = result.SetUnsatCore(core); err != nil {
			return lbool.Undef, err
		}
		proof, proofErr := s.kernel.Proof()
		if proofErr != nil {
			return lbool.Undef, proofErr
		}
		if err = result.SetProof(proof); err != nil {
			return lbool.Undef, err
		}
	}
	stats, err := result.Statistics()
	if err != nil {
		return lbool.Undef, err
	}
	collectSMTKernelStatistics(stats, s.kernel.Statistics())
	return status, nil
}

// PrepareCandidateModel mirrors smt_tactic's candidate-model allowance for
// conflict-limit, incomplete-theory, and incomplete-quantifier results.
func (s *SMTSolver) PrepareCandidateModel() (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	failure, err := s.kernel.LastFailure()
	if err != nil {
		return false, err
	}
	switch failure {
	case SMTFailureConflictLimit, SMTFailureTheory, SMTFailureQuantifiers:
	default:
		return false, nil
	}
	if !s.produceModels {
		return true, nil
	}
	if s.kernel.GeneratedModel() == nil {
		if err = s.kernel.buildGeneratedModel(); err != nil {
			return false, err
		}
	}
	if s.result == nil {
		return false, ast.ErrASTHandle
	}
	if err = s.result.SetModel(s.kernel.GeneratedModel()); err != nil {
		return false, err
	}
	return true, nil
}

func (s *SMTSolver) CheckSat(assumptions []ast.ASTHandle) (lbool.LBool, error) {
	return s.named.CheckSat(assumptions)
}

func (s *SMTSolver) NumAssertions() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return uint32(len(s.kernel.Assertions())), nil
}

func (s *SMTSolver) Assertion(index uint32) (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	assertions := s.kernel.Assertions()
	if uint64(index) >= uint64(len(assertions)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return assertions[index].Formula, nil
}

func (s *SMTSolver) NumAssumptions() (uint32, error) { return s.named.NumAssumptions() }
func (s *SMTSolver) Assumption(index uint32) (ast.ASTHandle, error) {
	return s.named.Assumption(index)
}

func (s *SMTSolver) GetConsequences(assumptions []ast.ASTHandle, variables []ast.ASTHandle) (status lbool.LBool, consequences []ast.ASTHandle, err error) {
	if err := s.open(); err != nil {
		return lbool.Undef, nil, err
	}
	if err := s.clearConsequences(); err != nil {
		return lbool.Undef, nil, err
	}
	if s.config.ModelEvaluator.Context == nil {
		return lbool.Undef, nil, ast.ErrASTDomain
	}
	previous := s.produceModels
	if err = s.SetProduceModels(true); err != nil {
		return lbool.Undef, nil, err
	}
	defer func() { err = errors.Join(err, s.SetProduceModels(previous)) }()
	return solver.DefaultGetConsequences(s, assumptions, variables, func(md *model.ModelCore) (solver.ConsequenceEvaluator, error) {
		config := s.config.ModelEvaluator
		config.Params = &s.params
		return model.NewModelEvaluator(md, config)
	})
}

func (s *SMTSolver) FindMutexes(variables []ast.ASTHandle) (lbool.LBool, [][]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, nil, err
	}
	state := s.kernel.internalizer
	literals := make([]uint32, 0, len(variables))
	for _, expression := range variables {
		atom := expression
		negative := false
		isNot, argument, err := s.manager.IsNot(expression)
		if err != nil {
			return lbool.Undef, nil, err
		}
		if isNot {
			atom, negative = argument, true
		}
		variable, found := state.boolVar[atom]
		if !found {
			continue
		}
		literals = append(literals, uint32(NewSMTLiteral(variable, negative).Index()))
	}
	finder := cliques.New()
	// The retained internalizer still stores binary clauses in the ordinary
	// two-watch region. Feed those same clause edges to max_cliques directly.
	for index := range state.clauses {
		clause := state.clauses[index].clause
		if clause == nil || clause.Deleted() || len(clause.Literals()) != 2 {
			continue
		}
		pair := clause.Literals()
		if err := finder.AddEdge(uint32(pair[0].Index()), uint32(pair[1].Index())); err != nil {
			return lbool.Undef, nil, err
		}
	}
	for index := range state.watches {
		for _, watched := range state.watches[index].Literals() {
			left, right := uint32(index)^1, uint32(watched.Index())
			if left < right {
				if err := finder.AddEdge(left, right); err != nil {
					return lbool.Undef, nil, err
				}
			}
		}
	}
	rows, err := finder.Cliques(literals)
	if err != nil {
		return lbool.Undef, nil, err
	}
	mutexes := make([][]ast.ASTHandle, len(rows))
	for i, row := range rows {
		mutexes[i] = make([]ast.ASTHandle, len(row))
		for j, index := range row {
			literal := SMTLiteralFromIndex(int32(index))
			if literal.Var() < SMTFirstBoolVar || int(literal.Var()) >= len(state.boolExpr) {
				return lbool.Undef, nil, ast.ErrASTDomain
			}
			expression := state.boolExpr[literal.Var()]
			if literal.Sign() {
				expression, err = s.manager.MkNot(expression)
				if err != nil {
					return lbool.Undef, nil, err
				}
			}
			mutexes[i][j] = expression
		}
	}
	return lbool.True, mutexes, nil
}

func (s *SMTSolver) PreferredSAT(preferred []ast.ASTHandle) (lbool.LBool, [][]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, nil, err
	}
	status, err := s.CheckSat(nil)
	if err != nil || status != lbool.True || len(preferred) == 0 {
		return status, nil, err
	}
	allowed := make(map[ast.ASTHandle]struct{}, len(preferred))
	for _, expression := range preferred {
		allowed[expression] = struct{}{}
	}
	accepted := make([]ast.ASTHandle, 0, len(preferred))
	cores := make([][]ast.ASTHandle, 0)
	for _, expression := range preferred {
		trial := append(append([]ast.ASTHandle(nil), accepted...), expression)
		status, err = s.CheckSat(trial)
		if err != nil || status == lbool.Undef {
			return status, cores, err
		}
		if status == lbool.True {
			accepted = append(accepted, expression)
			continue
		}
		core, coreErr := s.UnsatCore()
		if coreErr != nil {
			return lbool.Undef, cores, coreErr
		}
		filtered := make([]ast.ASTHandle, 0, len(core))
		for _, member := range core {
			if _, found := allowed[member]; found {
				filtered = append(filtered, member)
			}
		}
		if len(filtered) != 0 {
			cores = append(cores, filtered)
		}
	}
	status, err = s.CheckSat(accepted)
	return status, cores, err
}

func (s *SMTSolver) SetProgressCallback(callback solver.ProgressCallback) error {
	if err := s.open(); err != nil {
		return err
	}
	s.callback = callback
	return s.kernel.SetProgressCallback(callback)
}

func (s *SMTSolver) DisplayCore(out io.Writer) error { return solver.DefaultDisplay(s, out, nil) }
func (s *SMTSolver) Display(out io.Writer, assumptions []ast.ASTHandle) error {
	return solver.DefaultDisplay(s, out, assumptions)
}

func (s *SMTSolver) Translate(manager *ast.ASTManager, params *uparams.ParamsRef) (solver.Solver, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	if level, err := s.ScopeLevel(); err != nil {
		return nil, err
	} else if level != 0 {
		return nil, z3exception.NewDefaultException("translation of contexts is only supported at base level")
	}
	config := s.config
	if err := config.Params.CheckServices(manager.Symbols, manager.NumeralService()); err != nil {
		if config.TargetConfig == nil {
			return nil, err
		}
		config, err = config.TargetConfig(manager)
		if err != nil {
			return nil, err
		}
	}
	logic := ""
	var err error
	if s.logic != (usymbol.Symbol{}) {
		logic, err = s.manager.Symbols.String(s.logic)
		if err != nil {
			return nil, err
		}
	}
	next, err := NewSMTSolver(manager, params, logic, config)
	if err != nil {
		return nil, err
	}
	// Kernel translation currently copies preprocessed assertions/converters;
	// preserve that existing path until original NRA snapshots are translated.
	next.kernel.nraInputsValid = false

	// context::copy_plugins preserves concrete arithmetic types, with fresh
	// solver data and the parameter owner chosen by each mk_fresh constructor.
	if err = next.kernel.copyArithmeticPlugin(s.kernel); err != nil {
		return nil, errors.Join(err, next.Close())
	}
	if err = next.kernel.copyBitVectorPlugin(s.kernel); err != nil {
		return nil, errors.Join(err, next.Close())
	}
	if err = next.kernel.copyFloatingPointPlugin(s.kernel); err != nil {
		return nil, errors.Join(err, next.Close())
	}
	if err = next.kernel.copyArrayPlugin(s.kernel); err != nil {
		return nil, errors.Join(err, next.Close())
	}
	if err = next.kernel.copySequencePlugin(s.kernel); err != nil {
		return nil, errors.Join(err, next.Close())
	}
	if err = next.kernel.copyDatatypePlugin(s.kernel); err != nil {
		return nil, errors.Join(err, next.Close())
	}
	if err = next.kernel.copyPseudoBooleanPlugin(s.kernel); err != nil {
		return nil, errors.Join(err, next.Close())
	}
	if err = next.SetProduceModels(s.produceModels); err != nil {
		_ = next.Close()
		return nil, err
	}
	translate := func(value ast.ASTHandle) (ast.ASTHandle, error) { return value, nil }
	var translation *ast.ASTTranslation
	if manager != s.manager {
		translation, err = ast.NewASTTranslation(s.manager, manager, false)
		if err != nil {
			_ = next.Close()
			return nil, err
		}
		defer translation.Close()
		translate = translation.Translate
	}
	for _, assertion := range s.kernel.Assertions() {
		formula, copyErr := translate(assertion.Formula)
		if copyErr != nil {
			_ = next.Close()
			return nil, copyErr
		}
		proof, copyErr := translate(assertion.Proof)
		if copyErr != nil {
			_ = next.Close()
			return nil, copyErr
		}
		if copyErr = next.kernel.Assert(formula, proof); copyErr != nil {
			_ = next.Close()
			return nil, copyErr
		}
	}
	for _, assertion := range s.tracked {
		formula, getErr := s.manager.RefGet(assertion.formula)
		if getErr != nil {
			_ = next.Close()
			return nil, getErr
		}
		formula, getErr = translate(formula)
		if getErr != nil {
			_ = next.Close()
			return nil, getErr
		}
		tag, getErr := translate(assertion.tag)
		if getErr != nil {
			_ = next.Close()
			return nil, getErr
		}
		if getErr = next.AssertTracked(formula, tag); getErr != nil {
			_ = next.Close()
			return nil, getErr
		}
	}
	return next, nil
}

func (s *SMTSolver) Close() error {
	if s == nil || s.self != s {
		return ast.ErrASTHandle
	}
	if s.closed {
		return nil
	}
	var result error
	result = errors.Join(result, s.clearConsequences())
	result = errors.Join(result, s.clearResult())
	result = errors.Join(result, s.named.Close())
	result = errors.Join(result, s.restoreTracked(0))
	result = errors.Join(result, s.ResetObjectives())
	result = errors.Join(result, s.kernel.Close())
	result = errors.Join(result, s.config.Params.Del(&s.params))
	if s.ownedGlobals != nil {
		result = errors.Join(result, s.ownedGlobals.Close())
	}
	if s.ownedCatalog != nil {
		result = errors.Join(result, s.ownedCatalog.Close())
	}
	if result == nil {
		s.named = nil
		s.kernel = nil
		s.callback = nil
		s.manager = nil
		s.ownedGlobals = nil
		s.ownedCatalog = nil
		s.closed = true
	}
	return result
}

type smtSolverFactory struct {
	config SMTSolverConfig
	closed bool
}

func NewSMTSolverFactory(config SMTSolverConfig) (solver.Factory, error) {
	if config.Params == nil {
		return nil, uparams.ErrParamsOwner
	}
	return &smtSolverFactory{config: config}, nil
}

func (f *smtSolverFactory) New(m *ast.ASTManager, p *uparams.ParamsRef, _, models, _ bool, logic string) (solver.Solver, error) {
	if f == nil || f.closed {
		return nil, arena.ErrClosed
	}
	value, err := NewSMTSolver(m, p, logic, f.config)
	if err != nil {
		return nil, err
	}
	if err = value.SetProduceModels(models); err != nil {
		_ = value.Close()
		return nil, err
	}
	return value, nil
}

func (f *smtSolverFactory) Close() error {
	if f == nil || f.closed {
		return nil
	}
	f.closed = true
	return nil
}
