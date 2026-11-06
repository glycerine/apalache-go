package qe

// Source: src/qe/nlqsat.cpp:628-713,817-908.

import (
	"errors"
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/nlsat"
	nlsattactic "github.com/glycerine/g3/pkg/nlsat/tactic"
	"github.com/glycerine/g3/pkg/tactic"
	tacticcore "github.com/glycerine/g3/pkg/tactic/core"
	"github.com/glycerine/g3/pkg/util/lbool"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	primegen "github.com/glycerine/g3/pkg/util/prime"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type NLQSATTactic struct {
	self        *NLQSATTactic
	manager     *ast.ASTManager
	config      Config
	mode        qsatMode
	statistics  map[string]float64
	params      uparams.ParamsRef
	paramsReady bool
	bytes       uint64
	refs        uint32
	running     bool
	closed      bool
}

var _ tactic.Tactic = (*NLQSATTactic)(nil)

func NewNLQSATTactic(manager *ast.ASTManager, config Config) (*NLQSATTactic, error) {
	return NewNLQSATTacticWithParams(manager, nil, config)
}

func NewNLQSATTacticWithParams(manager *ast.ASTManager, params *uparams.ParamsRef, config Config) (*NLQSATTactic, error) {
	return newNLQTactic(manager, params, config, qsatSAT)
}

func NewNLQETactic(manager *ast.ASTManager, config Config) (*NLQSATTactic, error) {
	return NewNLQETacticWithParams(manager, nil, config)
}

func NewNLQETacticWithParams(manager *ast.ASTManager, params *uparams.ParamsRef, config Config) (*NLQSATTactic, error) {
	return newNLQTactic(manager, params, config, qsatQE)
}

func newNLQTactic(manager *ast.ASTManager, params *uparams.ParamsRef, config Config, mode qsatMode) (*NLQSATTactic, error) {
	if manager == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Cooperation == nil {
		return nil, arena.ErrConfig
	}
	if mode != qsatSAT && mode != qsatQE {
		return nil, ast.ErrASTDomain
	}
	bytes := uint64(reflect.TypeOf(NLQSATTactic{}).Size())
	if err := manager.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	t := &NLQSATTactic{manager: manager, config: config, mode: mode, bytes: bytes}
	t.self = t
	value, err := config.Context.NewValue()
	if err != nil {
		manager.Budget().Release(bytes)
		return nil, err
	}
	t.params, t.paramsReady = value, true
	if params != nil {
		if err = config.Context.Assign(&t.params, params); err != nil {
			return nil, errors.Join(err, config.Context.Del(&t.params), t.releaseHeader())
		}
	}
	return t, nil
}

func (t *NLQSATTactic) releaseHeader() error {
	if t.bytes != 0 {
		t.manager.Budget().Release(t.bytes)
		t.bytes = 0
	}
	return nil
}

func (t *NLQSATTactic) open() error {
	if t == nil || t.self != t || t.manager == nil {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	if t.running {
		return ast.ErrASTPending
	}
	return t.config.Context.CheckServices(t.manager.Symbols, t.manager.NumeralService())
}
func (t *NLQSATTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *NLQSATTactic) DecRef() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.refs == 0 {
		return arena.ErrBounds
	}
	t.refs--
	if t.refs == 0 {
		return t.Close()
	}
	return nil
}
func (t *NLQSATTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.running {
		return ast.ErrASTPending
	}
	if t.paramsReady {
		if err := t.config.Context.Del(&t.params); err != nil {
			return err
		}
		t.paramsReady = false
	}
	t.releaseHeader()
	t.closed = true
	return nil
}

func (t *NLQSATTactic) solverConfig(polynomials *polynomial.MultivariatePolynomialManager, algebraic *polynomial.AlgebraicManager) (nlsat.SolverConfig, error) {
	uintParameter := func(name string, fallback uint32) (uint32, error) {
		return t.config.Context.GetUIntName(&t.params, &name, fallback)
	}
	boolParameter := func(name string, fallback bool) (bool, error) {
		return t.config.Context.GetBoolName(&t.params, &name, fallback)
	}
	megabytes, err := uintParameter("max_memory", math.MaxUint32)
	if err != nil {
		return nlsat.SolverConfig{}, err
	}
	lazy, err := uintParameter("lazy", 0)
	if err != nil {
		return nlsat.SolverConfig{}, err
	}
	maxConflicts, err := uintParameter("max_conflicts", math.MaxUint32)
	if err != nil {
		return nlsat.SolverConfig{}, err
	}
	simplify, err := boolParameter("simplify_conflicts", true)
	if err != nil {
		return nlsat.SolverConfig{}, err
	}
	minimize, err := boolParameter("minimize_conflicts", false)
	if err != nil {
		return nlsat.SolverConfig{}, err
	}
	randomize, err := boolParameter("randomize", true)
	if err != nil {
		return nlsat.SolverConfig{}, err
	}
	reorder, err := boolParameter("reorder", true)
	if err != nil {
		return nlsat.SolverConfig{}, err
	}
	shuffle, err := boolParameter("shuffle_vars", false)
	if err != nil {
		return nlsat.SolverConfig{}, err
	}
	seed, err := uintParameter("seed", 0)
	if err != nil {
		return nlsat.SolverConfig{}, err
	}
	inlineVariables, err := boolParameter("inline_vars", false)
	if err != nil {
		return nlsat.SolverConfig{}, err
	}
	return nlsat.SolverConfig{
		Algebraic: algebraic, Polynomial: polynomials, Limit: &t.manager.Limit,
		MaxMemory: umemory.MegabytesToBytes(megabytes), Lazy: lazy,
		SimplifyCore: simplify, MinimizeCore: minimize, Randomize: randomize,
		Factor: false, InlineVars: inlineVariables, Reorder: reorder,
		ShuffleVars: shuffle, Seed: seed, MaxConflicts: maxConflicts,
	}, nil
}

func (t *NLQSATTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	name := "nlqsat"
	if t.mode == qsatQE {
		name = "nlqe"
	}
	if err = tactic.FailIfProofGeneration(name, goal); err != nil {
		return err
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	t.running = true
	defer func() { t.running = false }()
	forms, err := goal.AppendFormulas(nil)
	if err != nil {
		return err
	}
	formula, err := t.manager.MkAndArray(forms)
	if err != nil {
		return err
	}
	if t.mode == qsatQE {
		formula, err = t.manager.MkNot(formula)
		if err != nil {
			return err
		}
	}
	formulaRef, err := t.manager.NewRef(formula)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&formulaRef)) }()
	preparation, err := newQSATPreparation(t.manager, t.config)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, preparation.Close()) }()
	if err = preparation.hoist(t.mode, formulaRef); err != nil {
		return err
	}
	if err = preparation.ackermannizeDivisions(formulaRef); err != nil {
		return err
	}
	formula, err = t.manager.RefGet(formulaRef)
	if err != nil {
		return err
	}
	boolean, err := t.manager.MkBoolSort()
	if err != nil {
		return err
	}
	truth, err := t.manager.MkFreshConst("nlqsat-is-true", boolean)
	if err != nil {
		return err
	}
	definition, err := t.manager.MkIff(truth, formula)
	if err != nil {
		return err
	}
	cnfGoal, err := tactic.NewGoal(t.manager, false, false, false)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, cnfGoal.Close()) }()
	if err = cnfGoal.Assert(definition); err != nil {
		return err
	}
	services := model.ModelEvaluatorConfig{Context: t.config.Context, Globals: t.config.Globals, Memory: t.manager.NumeralService().RuntimeOwner(), Cooperation: t.config.Cooperation, Trace: t.config.Trace}
	cnf, err := tacticcore.MakeTseitinCNFTactic(t.manager, t.config.Context, t.config.Globals, &t.params, services)
	if err != nil {
		return err
	}
	if err = cnf.IncRef(); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, cnf.DecRef()) }()
	cnfResult := &tactic.TacticResult{}
	defer func() { err = errors.Join(err, cnfResult.Close()) }()
	if err = cnf.Apply(cnfGoal, cnfResult); err != nil {
		return err
	}

	algebraic, err := t.manager.ArithmeticAlgebraicManager()
	if err != nil {
		return err
	}
	rationals, err := algebraic.RationalManager()
	if err != nil {
		return err
	}
	primes, err := primegen.NewGenerator(t.manager.Budget())
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, primes.Close()) }()
	polynomials, err := polynomial.NewMultivariatePolynomialManager(polynomial.MultivariatePolynomialManagerConfig{
		Core:          polynomial.MultivariatePolynomialConfig{Integer: rationals.Integers(), Rationals: rationals, Checkpoint: t.manager.Limit.Inc, Cooperation: func() error { return t.config.Cooperation.Cooperate("nlqsat") }},
		Factorization: polynomial.UPolynomialFactorizationConfig{Primes: primes}, Trace: t.config.Trace,
	})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, polynomials.CloseChecked()) }()
	solverConfig, err := t.solverConfig(polynomials, algebraic)
	if err != nil {
		return err
	}
	solver, err := nlsat.NewSolver(solverConfig)
	if err != nil {
		return err
	}
	defer func() {
		if stats, cause := solver.Statistics(); cause == nil {
			if t.statistics == nil {
				t.statistics = make(map[string]float64)
			}
			for name, value := range stats {
				t.statistics[name] = value
			}
		} else {
			err = errors.Join(err, cause)
		}
		err = errors.Join(err, solver.Close())
	}()
	if err = solver.SetSignedProjection(true); err != nil {
		return err
	}
	compiler, err := nlsattactic.NewGoalCompiler(t.manager, solver)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, compiler.Close()) }()
	if err = compiler.SetFactor(false); err != nil {
		return err
	}
	if err = compiler.Compile(cnfGoal); err != nil {
		return tactic.NewTacticException(err.Error())
	}
	search, err := newNLQSearchFromCompilerMode(solver, compiler, preparation.variables, truth, t.mode)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, search.close()) }()
	status, err := search.check()
	if t.statistics == nil {
		t.statistics = make(map[string]float64)
	}
	t.statistics["qsat num rounds"] = float64(search.rounds)
	if err != nil {
		return tactic.NewTacticException(err.Error())
	}
	if status == lbool.Undef {
		return tactic.NewTacticException("search failed")
	}
	var answer ast.ASTRef
	if t.mode == qsatQE {
		if status != lbool.False {
			return tactic.NewTacticException("nonlinear elimination search failed")
		}
		answer, err = search.answer()
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, t.manager.DelRef(&answer)) }()
	}
	if status == lbool.True && t.mode == qsatSAT {
		models, cause := goal.ModelsEnabled()
		if cause != nil {
			return cause
		}
		if models {
			free := make([]ast.ASTHandle, len(preparation.freeVariables))
			for i, ref := range preparation.freeVariables {
				free[i], cause = t.manager.RefGet(ref)
				if cause != nil {
					return cause
				}
			}
			if len(free) != 0 {
				if cause = search.state.restoreAnswerModel(); cause != nil {
					return cause
				}
			}
			modelValue, supported, cause := compiler.ModelForExpressions(free)
			if cause != nil {
				return cause
			}
			if supported && modelValue != nil {
				var held tactic.ModelRef
				if cause = held.Set(modelValue.Core()); cause != nil {
					return errors.Join(cause, modelValue.Close())
				}
				defer func() { err = errors.Join(err, held.Close()) }()
				converter, createErr := tactic.NewStoredModel(modelValue.Core())
				if createErr != nil {
					return createErr
				}
				if cause = result.SetModelConverter(converter); cause != nil {
					return errors.Join(cause, converter.Close())
				}
			}
		}
	}
	if err = goal.Reset(); err != nil {
		return err
	}
	if t.mode == qsatQE {
		formula, cause := t.manager.RefGet(answer)
		if cause != nil {
			return cause
		}
		if err = goal.Assert(formula); err != nil {
			return err
		}
	} else if status == lbool.False {
		falsity, cause := t.manager.MkFalse()
		if cause != nil {
			return cause
		}
		if err = goal.Assert(falsity); err != nil {
			return err
		}
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	return result.AddGoal(goal)
}

func (t *NLQSATTactic) UpdateParams(params *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if params == nil {
		empty := t.config.Context.EmptyValue()
		params = &empty
	}
	return t.config.Context.Assign(&t.params, params)
}
func (t *NLQSATTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *NLQSATTactic) CollectStatistics() map[string]float64 {
	out := make(map[string]float64, len(t.statistics))
	for k, v := range t.statistics {
		out[k] = v
	}
	return out
}
func (t *NLQSATTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.statistics = nil
	return nil
}
func (t *NLQSATTactic) Cleanup() error                         { return t.open() }
func (t *NLQSATTactic) Reset() error                           { return t.Cleanup() }
func (t *NLQSATTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *NLQSATTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *NLQSATTactic) Translate(manager *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return newNLQTactic(manager, &t.params, t.config, t.mode)
}
