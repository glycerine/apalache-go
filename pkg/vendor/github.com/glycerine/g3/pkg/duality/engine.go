package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/solver"
	"io"
	"os"
)

// EngineConfig borrows the native solvers, factory, interpolation manager and
// evaluator services. Native must produce proofs; arithmetic interpolation
// requires eq2ineq normalization before assertions so certificates use the
// supported inequality rules. Theory expressions are copied. A nil Quantifiers asks
// the engine to own its qe-lite service; a supplied service stays borrowed.
type EngineConfig struct {
	ReportOutput      io.Writer
	Native, Auxiliary solver.Solver
	Factory           solver.Factory
	Evaluator         model.ModelEvaluatorConfig
	Simplifier        *interp.Manager
	Quantifiers       *qe.Lite
	Theory            []*Expr
	Profiler          *Profiler
	Interpolation     *interp.RunConfig
	Extensional       bool
}

// Engine owns the unwinding, counterexample, history proposals and per-solve
// checkers/heuristic. The source graph and configured services stay borrowed.
// Source: duality_solver.cpp:324-377,421-433, active lifecycle switches.
type Engine struct {
	conjectureFile           *os.File
	self                     *Engine
	source                   *RPFP
	config                   EngineConfig
	options                  SolverOptions
	graph                    *RPFP
	unwinding                *Unwinding
	counterexample           *Counterexample
	local, global            *LocalEdgeChecker
	heuristic                *ExpansionHeuristic
	proposers                []*HistoryProposer
	theory                   []*Expr
	cases                    *CaseInterpolator
	quantifiers              *qe.Lite
	ownsQuantifiers          bool
	learnedReplay            bool
	running, closing, closed bool
}

func NewEngine(source *RPFP, config EngineConfig) (out *Engine, result error) {
	if err := source.open(); err != nil {
		return nil, err
	}
	if config.Native == nil || config.Auxiliary == nil || config.Factory == nil || config.Simplifier == nil {
		return nil, ErrWrapperOwner
	}
	for _, native := range []solver.Solver{config.Native, config.Auxiliary} {
		m, err := native.Manager()
		if err != nil {
			return nil, err
		}
		if m != source.context.manager {
			return nil, ErrContextMismatch
		}
	}
	m, err := config.Simplifier.ASTManager()
	if err != nil {
		return nil, err
	}
	if m != source.context.manager {
		return nil, ErrContextMismatch
	}
	// The source interpolator obtains this factory from global solver state.
	run := interp.RunConfig{}
	if config.Interpolation != nil {
		run = *config.Interpolation
	}
	// The Duality report appends this profiler's interpolation tree. Supply
	// the same owner to interpolation unless the caller chose another one.
	if run.Profiler == nil && config.Profiler != nil {
		run.Profiler = config.Profiler.interpolation
	}
	if run.SolverFactory == nil {
		run.SolverFactory = config.Factory
	}
	config.Interpolation = &run
	e := &Engine{source: source, config: config, options: *NewSolverOptions(), counterexample: NewCounterexample()}
	e.self = e
	defer func() {
		if result != nil {
			result = errors.Join(result, e.Close())
		}
	}()
	for _, term := range config.Theory {
		if err = term.open(); err != nil {
			return nil, err
		}
		if term.value.object.context != source.context {
			return nil, ErrContextMismatch
		}
		copy, err := term.Clone()
		if err != nil {
			return nil, err
		}
		e.theory = append(e.theory, copy)
	}
	e.config.Theory = nil
	e.quantifiers = config.Quantifiers
	if e.quantifiers == nil {
		e.ownsQuantifiers = true
		e.quantifiers, err = qe.NewLite(m, nil, true, qe.Config{Context: source.context.config.params, Globals: config.Evaluator.Globals, Cooperation: config.Evaluator.Cooperation})
		if err != nil {
			return nil, err
		}
		e.ownsQuantifiers = true
	}
	e.cases = &CaseInterpolator{Auxiliary: config.Auxiliary, Extensional: config.Extensional, Simplifier: config.Simplifier, Quantifiers: e.quantifiers, Theory: e.theory}
	return e, nil
}
func (e *Engine) open() error {
	if e == nil || e.self != e || e.closing || e.closed || e.running {
		return ErrWrapperOwner
	}
	return e.source.open()
}
func (e *Engine) SetOption(name, value string) bool {
	return e.open() == nil && e.options.SetOption(name, value)
}
func (e *Engine) GetCounterexample() *Counterexample {
	if e == nil || e.self != e || e.closing || e.closed {
		return nil
	}
	return e.counterexample
}

// InvalidateCounterexampleView retires the counterexample wrapper previously
// returned by GetCounterexample while preserving its owned derivation for a
// subsequent LearnFrom. This is the Go counterpart of the fixedpoint
// interface swapping its published counterexample back into the old solver
// before constructing the next query.
func (e *Engine) InvalidateCounterexampleView() error {
	if err := e.open(); err != nil {
		return err
	}
	owned := NewCounterexample()
	if err := owned.Swap(e.counterexample); err != nil {
		return err
	}
	e.counterexample = owned
	return nil
}
func (e *Engine) UnwindingGraph() *RPFP {
	if e == nil || e.self != e || e.closing || e.closed {
		return nil
	}
	return e.graph
}
func (e *Engine) IsResultRecursionBounded() bool {
	return e != nil && e.self == e && !e.closing && !e.closed && e.unwinding.IsResultRecursionBounded()
}

func (e *Engine) closeRunServices() error {
	if e.heuristic != nil {
		if err := e.heuristic.Close(); err != nil {
			return err
		}
		e.heuristic = nil
	}
	if e.local != nil {
		if err := e.local.Close(); err != nil {
			return err
		}
		e.local = nil
	}
	if e.global != nil {
		if err := e.global.Close(); err != nil {
			return err
		}
		e.global = nil
	}
	if e.conjectureFile != nil {
		if e.graph != nil {
			e.graph.conjectureReporter = nil
		}
		file := e.conjectureFile
		e.conjectureFile = nil
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}
func (e *Engine) closeProposers() error {
	for len(e.proposers) > 0 {
		i := len(e.proposers) - 1
		if err := e.proposers[i].Close(); err != nil {
			return err
		}
		e.proposers = e.proposers[:i]
	}
	return nil
}

// Solve creates a fresh unwinding per call, replays an existing counterexample
// when present, and leaves only the resulting unwinding/counterexample owned
// after per-run inference resources and history proposals have been closed.
func (e *Engine) Solve() (satisfied bool, result error) {
	if err := e.open(); err != nil {
		return false, err
	}
	if err := e.source.checkpoint(); err != nil {
		return false, err
	}
	previousProfile := e.source.context.activeProfiler
	e.source.context.activeProfiler = e.config.Profiler
	defer func() { e.source.context.activeProfiler = previousProfile }()
	e.running = true
	defer func() {
		if result != nil && e.learnedReplay && e.heuristic != nil {
			result = errors.Join(result, e.heuristic.restoreReplay(e.counterexample))
		}
		result = errors.Join(result, e.closeRunServices(), e.closeProposers())
		e.running = false
	}()
	if err := e.closeRunServices(); err != nil {
		return false, err
	}
	if e.graph != nil {
		if err := e.graph.Close(); err != nil {
			return false, err
		}
		e.graph = nil
		e.unwinding = nil
	}
	graph, err := NewRPFPWithVocabulary(e.source.vocabulary)
	if err != nil {
		return false, err
	}
	e.graph = graph
	if e.options.Report {
		output := e.config.ReportOutput
		if output == nil {
			output = os.Stdout
		}
		graph.streamReporter = &StreamReporter{Format: ConjectureReporter{Output: output, Params: e.source.context.config.params, Globals: e.config.Evaluator.Globals}, depth: -1}
	}
	if e.options.ConjectureFile != "" {
		e.conjectureFile, err = os.Create(e.options.ConjectureFile)
		if err != nil {
			return false, err
		}
		graph.conjectureReporter = &ConjectureReporter{Output: e.conjectureFile, Params: e.source.context.config.params, Globals: e.config.Evaluator.Globals}
	}
	graph.HornClauses = e.source.HornClauses
	covering, err := NewCovering(graph, e.config.Auxiliary)
	if err != nil {
		return false, err
	}
	u, err := NewUnwinding(e.source, graph, covering)
	if err != nil {
		return false, err
	}
	e.unwinding = u
	u.FeasibleEdges = e.options.FeasibleEdges
	u.StratifiedInlining = e.options.StratifiedInlining
	u.RecursionBound = int(e.options.RecursionBound)
	for _, node := range e.source.nodes {
		if node.RecursionBound < ^uint32(0) {
			u.RecursionBound = 0
		}
	}
	if e.counterexample.Tree() != nil {
		e.heuristic, err = NewReplayExpansionHeuristic(graph, e.counterexample)
	} else {
		e.heuristic, err = NewExpansionHeuristic(graph)
	}
	if err != nil {
		return false, err
	}
	if err = e.counterexample.Clear(); err != nil {
		return false, err
	}
	e.local, err = NewLocalEdgeChecker(e.source, e.config.Factory, e.theory)
	if err != nil {
		return false, err
	}
	e.global, err = NewLocalEdgeChecker(e.source, e.config.Factory, e.theory)
	if err != nil {
		return false, err
	}
	if err = u.CreateEdgesByChildMap(); err != nil {
		return false, err
	}
	search := &UnwindingSearch{Unwinding: u, Native: e.config.Native, Evaluator: e.config.Evaluator, Local: e.local, Global: e.global, Auxiliary: e.config.Auxiliary, Simplifier: e.config.Simplifier, Cases: e.cases, Config: e.config.Interpolation, Heuristic: e.heuristic, Counterexample: e.counterexample, Proposers: e.proposers, FullExpand: e.options.FullExpand, UseUnderapprox: e.options.UseUnderapprox, NoConjectures: e.options.NoConj, EnableRestarts: e.options.EnableRestarts}
	if err = search.CreateInitialUnwinding(); err != nil {
		return false, err
	}
	timer, err := startProfile(e.source.context, "SolveMain")
	if err != nil {
		return false, err
	}
	safe, err := search.SolveMain()
	err = errors.Join(err, timer.Close())
	if err != nil {
		return false, err
	}
	if !safe && e.counterexampleHasQuantifiedTheory() {
		if err := e.validateQuantifiedCounterexample(); err != nil {
			return false, err
		}
	}
	e.learnedReplay = false
	return safe, nil
}

// LearnFrom swaps counterexamples and caches translated history facts from the
// other engine's completed unwinding. Both configured contexts/services must
// remain live while a transferred counterexample is used for replay.
func (e *Engine) LearnFrom(other *Engine) error {
	if err := e.open(); err != nil {
		return err
	}
	if err := other.open(); err != nil {
		return err
	}
	if other.graph == nil {
		return ErrWrapperOwner
	}
	proposer, err := NewHistoryProposer(other.graph, e.source)
	if err != nil {
		return err
	}
	hasReplay := other.counterexample.Tree() != nil
	if err := e.counterexample.Swap(other.counterexample); err != nil {
		return errors.Join(err, proposer.Close())
	}
	e.learnedReplay = hasReplay
	e.proposers = append(e.proposers, proposer)
	return nil
}

// RollbackLearnFrom returns an unconsumed replay guide to the engine that
// supplied it. History proposals remain owned by this engine and are released
// with its failed run.
func (e *Engine) RollbackLearnFrom(other *Engine) error {
	if err := e.open(); err != nil {
		return err
	}
	if err := other.open(); err != nil {
		return err
	}
	if !e.learnedReplay {
		return nil
	}
	if other.counterexample.Tree() != nil {
		return ErrWrapperOwner
	}
	if err := other.counterexample.Swap(e.counterexample); err != nil {
		return err
	}
	e.learnedReplay = false
	return nil
}
func (e *Engine) Close() error {
	if e == nil {
		return nil
	}
	if e.self != e || e.running {
		return ErrWrapperOwner
	}
	if e.closed {
		return nil
	}
	e.closing = true
	if err := e.closeRunServices(); err != nil {
		return err
	}
	if err := e.closeProposers(); err != nil {
		return err
	}
	if err := e.counterexample.Close(); err != nil {
		return err
	}
	if e.graph != nil {
		if err := e.graph.Close(); err != nil {
			return err
		}
		e.graph = nil
		e.unwinding = nil
	}
	if e.ownsQuantifiers && e.quantifiers != nil {
		if err := e.quantifiers.Close(); err != nil {
			return err
		}
		e.quantifiers = nil
	}
	for len(e.theory) > 0 {
		i := len(e.theory) - 1
		err := e.theory[i].Close()
		if e.theory[i].Closed() {
			e.theory[i] = nil
			e.theory = e.theory[:i]
		}
		if err != nil {
			return err
		}
	}
	e.closed = true
	return nil
}
