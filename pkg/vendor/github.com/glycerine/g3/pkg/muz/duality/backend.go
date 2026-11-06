package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	graph "github.com/glycerine/g3/pkg/duality"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Backend owns the concrete Duality context, native solvers and query services.
// The AST manager and config's parameter/evaluator services stay borrowed.
// Source: duality_dl_interface.cpp duality_data/query construction/destruction.
type Backend struct {
	self              *Backend
	context           *graph.Context
	factory           solver.Factory
	native, auxiliary solver.Solver
	simplifier        *interp.Manager
	quantifiers       *qe.Lite
	queries           *QueryEngine
	closed            bool
}

func NewBackend(manager *ast.ASTManager, config smtcore.SMTSolverConfig, mbqi bool) (out *Backend, result error) {
	if manager == nil || config.Params == nil || config.Globals == nil || config.ModelEvaluator.Cooperation == nil || config.ModelEvaluator.Memory == nil {
		return nil, graph.ErrWrapperOwner
	}
	if config.ModelEvaluator.Context != config.Params || config.ModelEvaluator.Globals != config.Globals {
		return nil, graph.ErrContextMismatch
	}
	b := &Backend{}
	b.self = b
	attempt := &backendConstruction{backend: b, runtime: config.ModelEvaluator.Memory}
	defer func() { out, result = attempt.finish(result) }()
	cfg, err := graph.NewConfig(config.Params, nil)
	if err != nil {
		return nil, err
	}
	attempt.config = cfg
	if err := cfg.SetBool("mbqi", mbqi); err != nil {
		return nil, err
	}
	b.context, err = graph.NewContext(manager, cfg)
	if err != nil {
		return nil, err
	}
	b.factory, err = smtcore.NewSMTSolverFactory(config)
	if err != nil {
		return nil, err
	}
	b.native, err = NewNativeSolver(b.context, config.Params, b.factory, mbqi)
	if err != nil {
		return nil, err
	}
	b.auxiliary, err = NewNativeSolver(b.context, config.Params, b.factory, mbqi)
	if err != nil {
		return nil, err
	}
	b.simplifier, err = interp.NewManagerWithParams(manager, config.Params, config.Globals)
	if err != nil {
		return nil, err
	}
	b.quantifiers, err = qe.NewLite(manager, nil, true, qe.Config{Context: config.Params, Globals: config.Globals, Cooperation: config.ModelEvaluator.Cooperation})
	if err != nil {
		return nil, err
	}
	profile, interpolationProfile := graph.ProcessProfilers()
	b.queries, err = NewQueryEngine(b.context, graph.EngineConfig{
		Native: b.native, Auxiliary: b.auxiliary, Factory: b.factory, Evaluator: config.ModelEvaluator,
		Profiler: profile, Interpolation: &interp.RunConfig{Profiler: interpolationProfile},
		Simplifier: b.simplifier, Quantifiers: b.quantifiers, Extensional: mbqi,
	})
	if err != nil {
		return nil, err
	}
	return b, nil
}
func (b *Backend) open() error {
	if b == nil || b.self != b || b.closed || b.queries == nil {
		return graph.ErrWrapperOwner
	}
	return b.queries.open()
}
func (b *Backend) SetOptions(options QueryOptions) error {
	if err := b.open(); err != nil {
		return err
	}
	return b.queries.SetOptions(options)
}
func (b *Backend) Query(prepared *PreparedQuery) (lbool.LBool, error) {
	if err := b.open(); err != nil {
		return lbool.Undef, err
	}
	return b.queries.Query(prepared)
}

// Counterexample is borrowed until the next Query or Close.
func (b *Backend) Counterexample() *graph.Counterexample {
	if b == nil || b.self != b || b.closed || b.queries == nil {
		return nil
	}
	return b.queries.Counterexample()
}
func (b *Backend) IsResultRecursionBounded() bool {
	return b != nil && b.self == b && !b.closed && b.queries != nil && b.queries.IsResultRecursionBounded()
}
func (b *Backend) Close() error {
	if b == nil {
		return nil
	}
	if b.self != b {
		return graph.ErrWrapperOwner
	}
	if b.closed {
		return nil
	}
	if b.queries != nil {
		if err := b.queries.Close(); err != nil {
			return err
		}
		b.queries = nil
	}
	if b.quantifiers != nil {
		if err := b.quantifiers.Close(); err != nil {
			return err
		}
		b.quantifiers = nil
	}
	if b.simplifier != nil {
		if err := b.simplifier.Close(); err != nil {
			return err
		}
		b.simplifier = nil
	}
	if b.auxiliary != nil {
		if err := b.auxiliary.Close(); err != nil {
			return err
		}
		b.auxiliary = nil
	}
	if b.native != nil {
		if err := b.native.Close(); err != nil {
			return err
		}
		b.native = nil
	}
	if b.factory != nil {
		if err := b.factory.Close(); err != nil {
			return err
		}
		b.factory = nil
	}
	if b.context != nil {
		if err := b.context.Close(); err != nil {
			return err
		}
		b.context = nil
	}
	b.closed = true
	return nil
}
