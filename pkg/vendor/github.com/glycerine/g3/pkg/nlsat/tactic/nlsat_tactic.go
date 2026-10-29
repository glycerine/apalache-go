package tactic

// Source: src/nlsat/tactic/nlsat_tactic.cpp:27-256.

import (
	"errors"
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/nlsat"
	basetactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	primegen "github.com/glycerine/g3/pkg/util/prime"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// NLSATTactic stores only the source parameters. Solver, compiler, and
// polynomial state are scoped to one Apply call.
type NLSATTactic struct {
	self        *NLSATTactic
	manager     *ast.ASTManager
	context     *uparams.ParamsContext
	globals     *uparams.GlobalParamsService
	services    model.ModelEvaluatorConfig
	params      uparams.ParamsRef
	statistics  map[string]float64
	ownerBytes  uint64
	refs        uint32
	paramsReady bool
	running     bool
	closed      bool
}

var _ basetactic.Tactic = (*NLSATTactic)(nil)

func NewNLSATTactic(manager *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, params *uparams.ParamsRef, services model.ModelEvaluatorConfig) (_ *NLSATTactic, err error) {
	if manager == nil || context == nil || globals == nil || globals.Context != context || services.Context != context || services.Globals != globals || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err = context.CheckServices(manager.Symbols, manager.NumeralService()); err != nil {
		return nil, err
	}
	amount := uint64(reflect.TypeOf(NLSATTactic{}).Size())
	if err = manager.Budget().Reserve(amount); err != nil {
		return nil, err
	}
	t := &NLSATTactic{manager: manager, context: context, globals: globals, services: services, ownerBytes: amount}
	t.self = t
	defer func() {
		if err != nil {
			err = errors.Join(err, t.Close())
		}
	}()
	t.params, err = context.NewValue()
	t.paramsReady = err == nil
	if err == nil && params != nil {
		err = context.Assign(&t.params, params)
	}
	return t, err
}

func MakeNLSATTactic(manager *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, params *uparams.ParamsRef, services model.ModelEvaluatorConfig) (basetactic.Tactic, error) {
	child, err := NewNLSATTactic(manager, context, globals, params, services)
	if err != nil {
		return nil, err
	}
	wrapped, err := basetactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return wrapped, nil
}

func (t *NLSATTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || !t.paramsReady {
		return arena.ErrClosed
	}
	if t.running {
		return ast.ErrASTPending
	}
	return t.context.Check(&t.params)
}

func (t *NLSATTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}

func (t *NLSATTactic) DecRef() error {
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

func (t *NLSATTactic) Close() (err error) {
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
		if err = t.context.Del(&t.params); err != nil {
			return err
		}
		t.paramsReady = false
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}

func (t *NLSATTactic) Cleanup() error { return t.open() }
func (t *NLSATTactic) Reset() error   { return t.Cleanup() }

func (t *NLSATTactic) Translate(manager *ast.ASTManager) (basetactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewNLSATTactic(manager, t.context, t.globals, &t.params, t.services)
}

func (t *NLSATTactic) UpdateParams(params *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	return t.context.Assign(&t.params, params)
}

func (t *NLSATTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := uparams.InsertMaxMemory(out); err != nil {
		return err
	}
	entries := []struct {
		name, description string
		kind              uparams.ParamKind
	}{
		{"lazy", "how lazy the solver is.", uparams.ParamUInt},
		{"simplify_conflicts", "simplify conflicts using equalities before resolving them.", uparams.ParamBool},
		{"minimize_conflicts", "minimize conflicts.", uparams.ParamBool},
		{"randomize", "randomize selection of a witness.", uparams.ParamBool},
		{"max_conflicts", "maximum number of conflicts.", uparams.ParamUInt},
		{"factor", "factor polynomials produced during conflict resolution.", uparams.ParamBool},
	}
	for _, entry := range entries {
		if err := out.InsertName(entry.name, entry.kind, &entry.description, nil, nil); err != nil {
			return err
		}
	}
	return nil
}

func (t *NLSATTactic) CollectStatistics() map[string]float64 {
	result := make(map[string]float64, len(t.statistics))
	for name, value := range t.statistics {
		result[name] = value
	}
	return result
}
func (t *NLSATTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.statistics = nil
	return nil
}
func (t *NLSATTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *NLSATTactic) SetProgressCallback(func(uint32)) error { return t.open() }

func (t *NLSATTactic) solverConfig(polynomials *polynomial.MultivariatePolynomialManager, algebraic *polynomial.AlgebraicManager) (nlsat.SolverConfig, error) {
	uintParameter := func(name string, fallback uint32) (uint32, error) {
		return t.context.GetUIntName(&t.params, &name, fallback)
	}
	boolParameter := func(name string, fallback bool) (bool, error) {
		return t.context.GetBoolName(&t.params, &name, fallback)
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
	factor, err := boolParameter("factor", true)
	if err != nil {
		return nlsat.SolverConfig{}, err
	}
	return nlsat.SolverConfig{
		Algebraic: algebraic, Polynomial: polynomials, Limit: &t.manager.Limit,
		MaxMemory: umemory.MegabytesToBytes(megabytes), Lazy: lazy,
		SimplifyCore: simplify, MinimizeCore: minimize, Randomize: randomize, Factor: factor, MaxConflicts: maxConflicts,
	}, nil
}

func (t *NLSATTactic) Apply(goal *basetactic.Goal, result *basetactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	t.running = true
	defer func() { t.running = false }()
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	if goal.IsDecided() {
		return result.AddGoal(goal)
	}
	if err = basetactic.FailIfProofGeneration("nlsat", goal); err != nil {
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
	polynomials, err := polynomial.NewMultivariatePolynomialManager(polynomial.MultivariatePolynomialManagerConfig{
		Core: polynomial.MultivariatePolynomialConfig{
			Integer: rationals.Integers(), Rationals: rationals,
			Checkpoint:  t.manager.Limit.Inc,
			Cooperation: func() error { return t.services.Cooperation.Cooperate("nlsat") },
		},
		Factorization: polynomial.UPolynomialFactorizationConfig{Primes: primes},
		Trace:         t.services.Trace,
	})
	if err != nil {
		return errors.Join(err, primes.Close())
	}
	var solver *nlsat.Solver
	var compiler *GoalCompiler
	defer func() {
		if compiler != nil {
			err = errors.Join(err, compiler.Close())
		}
		if solver != nil {
			if statistics, cause := solver.Statistics(); cause != nil {
				err = errors.Join(err, cause)
			} else {
				t.statistics = statistics
			}
			err = errors.Join(err, solver.Close())
		}
		err = errors.Join(err, polynomials.CloseChecked())
		err = errors.Join(err, primes.Close())
	}()
	config, err := t.solverConfig(polynomials, algebraic)
	if err != nil {
		return err
	}
	solver, err = nlsat.NewSolver(config)
	if err != nil {
		return err
	}
	compiler, err = NewGoalCompiler(t.manager, solver)
	if err != nil {
		return err
	}
	if err = compiler.SetFactor(config.Factor); err != nil {
		return err
	}
	if err = compiler.Compile(goal); err != nil {
		return basetactic.NewTacticException(err.Error())
	}
	status, err := solver.Check()
	if err != nil {
		return basetactic.NewTacticException(err.Error())
	}
	switch status {
	case lbool.True:
		unsupported, cause := compiler.ContainsUnsupported()
		if cause != nil {
			return cause
		}
		if !unsupported {
			models, cause := goal.ModelsEnabled()
			if cause != nil {
				return cause
			}
			if models {
				modelValue, supported, cause := compiler.Model()
				if cause != nil {
					return cause
				}
				if !supported {
					break
				}
				var held basetactic.ModelRef
				if cause = held.Set(modelValue.Core()); cause != nil {
					return errors.Join(cause, modelValue.Close())
				}
				defer func() { err = errors.Join(err, held.Close()) }()
				converter, cause := basetactic.NewStoredModel(modelValue.Core())
				if cause != nil {
					return cause
				}
				if cause = result.SetModelConverter(converter); cause != nil {
					return errors.Join(cause, converter.Close())
				}
			}
			if err = goal.Reset(); err != nil {
				return err
			}
		}
	case lbool.False:
		var core basetactic.DependencyRef
		defer func() { err = errors.Join(err, core.Close()) }()
		cores, cause := goal.UnsatCoresEnabled()
		if cause != nil {
			return cause
		}
		if cores {
			if cause = compiler.Core(&core); cause != nil {
				return cause
			}
			if cause = result.SetCore(core.Get()); cause != nil {
				return cause
			}
		}
		falsity, cause := t.manager.MkFalse()
		if cause != nil {
			return cause
		}
		if cause = goal.AssertWithProof(falsity, ast.ASTHandle{}, core.Get()); cause != nil {
			return cause
		}
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	return result.AddGoal(goal)
}
