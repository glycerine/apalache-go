package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
	"io"
	"os"

	graph "github.com/glycerine/g3/pkg/duality"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// QueryEngine owns the fixedpoint query's RPFP and Duality solver across
// repeated queries. Context and solver services are borrowed and must outlive
// Close. Registration and rule preprocessing are performed by its caller.
// Source: duality_dl_interface.cpp, duality_data and query solver lifecycle.
type QueryEngine struct {
	profileOutput io.Writer
	options       QueryOptions
	self          *QueryEngine
	context       *graph.Context
	config        graph.EngineConfig
	active        *queryRun
	runs          []*queryRun
	status        lbool.LBool
	closing       bool
	closed        bool
}
type queryRun struct {
	graph      *graph.RPFP
	engine     *graph.Engine
	background []*graph.Expr
	scopes     []querySolverScope
}

func (r *queryRun) close() error {
	if err := r.closeScopes(); err != nil {
		return err
	}
	if r.engine != nil {
		if err := r.engine.Close(); err != nil {
			return err
		}
		r.engine = nil
	}
	if r.graph != nil {
		if err := r.graph.Close(); err != nil {
			return err
		}
		r.graph = nil
	}
	for len(r.background) > 0 {
		i := len(r.background) - 1
		err := r.background[i].Close()
		if r.background[i].Closed() {
			r.background[i] = nil
			r.background = r.background[:i]
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func NewQueryEngine(context *graph.Context, config graph.EngineConfig) (*QueryEngine, error) {
	if context == nil {
		return nil, graph.ErrWrapperOwner
	}
	if _, err := context.Manager(); err != nil {
		return nil, err
	}
	if config.Native == nil || config.Auxiliary == nil || config.Factory == nil || config.Simplifier == nil || config.Quantifiers == nil {
		return nil, graph.ErrWrapperOwner
	}
	if config.Profiler == nil {
		run := interp.RunConfig{}
		if config.Interpolation != nil {
			run = *config.Interpolation
		}
		if run.Profiler == nil {
			config.Profiler, run.Profiler = graph.ProcessProfilers()
		} else {
			config.Profiler = graph.NewProfiler(run.Profiler)
		}
		config.Interpolation = &run
	}
	e := &QueryEngine{context: context, config: config, status: lbool.Undef, options: DefaultQueryOptions()}
	e.self = e
	return e, nil
}
func (e *QueryEngine) open() error {
	if e == nil || e.self != e || e.closing || e.closed {
		return graph.ErrWrapperOwner
	}
	_, err := e.context.Manager()
	return err
}
func (e *QueryEngine) reap() error {
	var result error
	kept := e.runs[:0]
	for _, run := range e.runs {
		if run == e.active {
			kept = append(kept, run)
			continue
		}
		if err := run.close(); err != nil {
			result = errors.Join(result, err)
			kept = append(kept, run)
		}
	}
	e.runs = kept
	return result
}

// Query learns from the previous solver before solving the new prepared graph.
// Duality safety means fixedpoint UNSAT; a counterexample means SAT. The source
// active bounded-result branch also returns UNSAT for bounded safety.
func (e *QueryEngine) Query(prepared *PreparedQuery) (status lbool.LBool, result error) {
	if err := e.open(); err != nil {
		return lbool.Undef, err
	}
	// Every attempted query invalidates publication, including setup failures.
	e.status = lbool.Undef
	// The source swaps the published counterexample back into the old solver
	// before constructing the replacement problem. Rotate the Go wrapper at the
	// same boundary: the borrowed view is invalidated, while LearnFrom can still
	// consume the old solver's derivation as a replay guide.
	if e.active != nil {
		if err := e.active.engine.InvalidateCounterexampleView(); err != nil {
			return lbool.Undef, err
		}
	}
	if prepared == nil {
		return lbool.Undef, graph.ErrWrapperOwner
	}
	// A previous attempt may still own a refused background-scope Pop.
	// Drain it before either borrowed solver is used by another query.
	if err := e.reap(); err != nil {
		return lbool.Undef, err
	}
	manager, err := e.context.Manager()
	if err != nil {
		return lbool.Undef, err
	}
	proofScope, err := ast.NewScopedProof(manager)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() {
		result = errors.Join(result, proofScope.Close())
		if result != nil {
			status = lbool.Undef
			e.status = lbool.Undef
		}
	}()
	e.status = lbool.Undef
	nextGraph, err := prepared.BuildGraph(e.context, e.config.Simplifier, e.config.Quantifiers)
	if err != nil {
		return lbool.Undef, err
	}
	next := &queryRun{graph: nextGraph}
	e.runs = append(e.runs, next)
	defer func() {
		result = errors.Join(result, next.closeScopes())
		if result == nil {
			e.active = next
			e.status = status
		}
		// Refused scopes keep their run alive until the next Query or Close.
		// Its graph and engine depend on the same borrowed native solvers.
		if len(next.scopes) == 0 {
			result = errors.Join(result, e.reap())
		}
		if result != nil {
			status = lbool.Undef
			e.status = lbool.Undef
		}
	}()
	config := e.config
	config.Theory = append([]*graph.Expr(nil), config.Theory...)
	for _, ref := range prepared.background {
		h, err := manager.RefGet(ref)
		if err != nil {
			return lbool.Undef, err
		}
		axiom, err := graph.NewExpr(e.context, h)
		if err != nil {
			return lbool.Undef, err
		}
		next.background = append(next.background, axiom)
		config.Theory = append(config.Theory, axiom)
	}
	if len(config.Theory) > 0 {
		for _, native := range []solver.Solver{config.Native, config.Auxiliary} {
			if err := next.pushScope(native); err != nil {
				return lbool.Undef, err
			}
			for _, axiom := range config.Theory {
				h, err := axiom.Raw()
				if err != nil {
					return lbool.Undef, err
				}
				if err := native.Assert(h); err != nil {
					return lbool.Undef, err
				}
			}
		}
	}
	next.engine, err = graph.NewEngine(nextGraph, config)
	if err != nil {
		return lbool.Undef, err
	}
	if e.active != nil {
		if err := next.engine.LearnFrom(e.active.engine); err != nil {
			return lbool.Undef, err
		}
	}
	e.options.apply(next.engine)
	solved, err := next.engine.Solve()
	if err != nil {
		if e.active != nil {
			err = errors.Join(err, next.engine.RollbackLearnFrom(e.active.engine))
		}
		return lbool.Undef, err
	}
	status = lbool.True
	if solved {
		status = lbool.False
	}
	if e.options.Profile {
		output := e.profileOutput
		if output == nil {
			output = os.Stdout
		}
		if config.Profiler == nil {
			return lbool.Undef, graph.ErrWrapperOwner
		}
		if err := config.Profiler.Print(output); err != nil {
			return lbool.Undef, err
		}
	}
	return status, nil
}

// Counterexample returns a borrowed result, invalidated by Query or Close.
func (e *QueryEngine) Counterexample() *graph.Counterexample {
	if e == nil || e.self != e || e.closed || e.active == nil || e.status != lbool.True {
		return nil
	}
	return e.active.engine.GetCounterexample()
}
func (e *QueryEngine) IsResultRecursionBounded() bool {
	return e != nil && e.self == e && !e.closed && e.active != nil && e.status == lbool.False && e.active.engine.IsResultRecursionBounded()
}
func (e *QueryEngine) Close() error {
	if e == nil {
		return nil
	}
	if e.self != e {
		return graph.ErrWrapperOwner
	}
	if e.closed {
		return nil
	}
	e.closing = true
	e.active = nil
	if err := e.reap(); err != nil {
		return err
	}
	e.closed = true
	e.status = lbool.Undef
	return nil
}
