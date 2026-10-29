package sat

// Source: src/sat/sat_solver.cpp:34–74,2763–2771; src/sat/sat_solver.h:74–483; src/sat/sat_mus.cpp:28–30.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type SolverConfig struct {
	Debug         *diagnostic.DebugContext
	TraceTags     *diagnostic.TraceTags
	ParamsContext *uparams.ParamsContext
	Params        uparams.ParamsRef
	RLimit        *control.Reslimit
	Budget        *arena.Budget
	Runtime       *Runtime
}

// Named source members borrow the one actual variable/lifecycle/propagation
// graph. Cold contexts break constructor cycles without supplying completed
// source members, duplicated counters, or an alternative solver.
type Solver struct {
	dimacsOutput                 *utext.NumericTextStream // explicit borrowed stdout transport counterpart.
	blockedCorrectionSets        uint32                   // actual m_stats field; source has only reset/read sites.
	self                         *Solver
	runtime                      *Runtime
	context                      *uparams.ParamsContext
	Limit                        *control.Reslimit
	budget                       *arena.Budget
	traceTags                    *diagnostic.TraceTags
	config                       *satMainConfig
	Clauses                      *ClauseAllocator
	Variables                    *satSolverVariableState
	propagation                  *satSolverPropagationState
	Lifecycle                    *satSolverClauseLifecycle
	analysis                     *satSolverConflictAnalysis
	dynamic                      *satSolverDynamicSubsumption
	conflict                     *satSolverConflictResolution
	decision                     *satSolverDecision
	cleaner                      *satCleaner
	converter                    *satModelConverter
	simplifier                   *satSimplifier
	scc                          *satSCC
	asymm                        *satAsymmBranch
	probing                      *satSolverProbing
	gc                           *satSolverGC
	model                        satUintColumn
	mus                          satSolverMUSMembers
	params                       uparams.ParamsRef
	elimTodo                     *satSolverUintSet
	antecedents                  *satSolverIndexMap
	auxLiterals, todoAntecedents satUintColumn
	userBinClauses               []BinaryLiteralPair
	binaryClauseGraph            []satUintColumn
	closeSourceCause             error
	closeGuardAcknowledged       bool
	userGC                       satSolverUserGC
	nextSimplify                 uint32
	mutexFrames                  *satMutexFrame
	mutexRunning                 bool
	consequenceFrames            *satConsequenceFrame
	consequenceRunning           bool
	ownerBytes                   uint64
	closeStage                   uint32
	constructed, closing, closed bool
}

// Actual source MUS value members and constructor fields. Algorithms and
// named interrupted lexical ownership live in sat_solver_mus.go; optional
// report/TRACE producers remain separate source obligations.
type satSolverMUSMembers struct {
	solver           *Solver
	frames           *satMUSFrame
	core, mus, model satUintColumn
	isActive         bool
	maxNumRestarts   uint32
	constructed      bool
}

func NewSATSolver(c SolverConfig) (s *Solver, err error) {
	if c.Runtime == nil || c.ParamsContext == nil || c.Budget == nil || c.RLimit == nil || c.RLimit.Closed() {
		return nil, ErrSATOwner
	}
	if c.Debug != nil && !c.Debug.ValidOwner() {
		return nil, ErrSATOwner
	}
	if err = c.Runtime.open(); err != nil {
		return nil, err
	}
	if c.Runtime.context != c.ParamsContext || c.Runtime.budget != c.Budget || c.Runtime.memory.Budget() != c.Budget {
		return nil, ErrSATOwner
	}
	if err = c.ParamsContext.Check(&c.Params); err != nil {
		return nil, err
	}
	if c.Runtime.liveSolvers == ^uint32(0) {
		return nil, arena.ErrSlotLimit
	}
	n := uint64(reflect.TypeOf(Solver{}).Size())
	if err = c.Budget.Reserve(n); err != nil {
		return nil, err
	}
	s = &Solver{runtime: c.Runtime, context: c.ParamsContext, Limit: c.RLimit, budget: c.Budget, traceTags: c.TraceTags, ownerBytes: n}
	s.self = s
	c.Runtime.liveSolvers++
	defer func() {
		if err != nil {
			if e := s.Close(); e != nil {
				err = errors.Join(err, e)
			} else {
				s = nil
			}
		}
	}()
	// Source m_config is the first owning member. Its symbol/parameter body is
	// unchanged, followed by source stats reset and default m_rand construction.
	if s.config, err = newSATMainConfigWithService(c.Runtime.globals, c.Runtime.globalService, &c.Params, c.Budget); err != nil {
		return s, err
	}
	if s.decision, err = newSATColdDecision(c.Budget); err != nil {
		return s, err
	}
	s.decision.traceTags = c.TraceTags
	if s.Clauses, err = NewSATClauseAllocator(satClauseAllocatorConfig{Storage: arena.SmallBytesConfig{Descriptors: arena.Config{Budget: c.Budget}}, OOMRuntime: c.Runtime.memory}); err != nil {
		return s, err
	}
	s.Clauses.traceTags = c.TraceTags
	// The following actual charged contexts contain empty bound vector headers.
	// They are Go ownership metadata, not prematurely constructed source queue,
	// MC payloads, probing parameters, or additional malloc accounting events.
	if s.elimTodo, err = newSATSolverUintSet(c.Budget); err != nil {
		return s, err
	}
	vc := arena.VectorConfig{Descriptors: arena.Config{Budget: c.Budget}}
	if s.Variables, err = newSATColdVariableState(s.elimTodo, c.Budget, vc, c.Runtime.memory); err != nil {
		return s, err
	}
	if s.converter, err = newSATColdModelConverter(c.Budget); err != nil {
		return s, err
	}
	s.converter.traceTags = c.TraceTags
	if s.propagation, err = newSATSolverPropagationState(s.Variables, s.Clauses, s.converter, c.RLimit, c.Runtime.memory, s.config, c.Budget); err != nil {
		return s, err
	}
	s.propagation.traceTags = c.TraceTags
	if s.Lifecycle, err = newSATSolverClauseLifecycle(s.propagation, c.Budget); err != nil {
		return s, err
	}
	s.Lifecycle.traceTags = c.TraceTags
	if s.analysis, err = newSATSolverConflictAnalysis(s.propagation, c.Budget); err != nil {
		return s, err
	}
	if s.probing, err = newSATColdProbing(s.Lifecycle, c.Budget); err != nil {
		return s, err
	}
	if s.dynamic, err = newSATSolverDynamicSubsumption(s.analysis, s.probing, c.Budget); err != nil {
		return s, err
	}
	if s.conflict, err = newSATSolverConflictResolution(s.analysis, s.dynamic); err != nil {
		return s, err
	}
	s.conflict.traceTags = c.TraceTags
	s.decision.conflict = s.conflict
	// Declared source member construction resumes: cleaner, model, MC,
	// simplifier, SCC, asymmetric branching, probing, MUS.
	if s.cleaner, err = newSATCleaner(s.Lifecycle, c.Budget); err != nil {
		return s, err
	}
	s.cleaner.solver = s
	s.model.Bind(c.Runtime.memory)
	if err = s.converter.initializeSource(); err != nil {
		return s, err
	}
	if s.simplifier, err = newSATSimplifier(s.Lifecycle, s.cleaner, s.decision, &c.Params); err != nil {
		return s, err
	}
	s.simplifier.debug = c.Debug
	if s.scc, err = newSATSCC(s.Lifecycle, &c.Params); err != nil {
		return s, err
	}
	s.scc.solver = s
	s.scc.eliminator.solver = s
	if s.asymm, err = newSATAsymmBranch(s.Lifecycle, &c.Params); err != nil {
		return s, err
	}
	s.asymm.solver = s
	if err = s.probing.initializeSource(c.ParamsContext, &c.Params); err != nil {
		return s, err
	}
	s.mus.core.Bind(c.Runtime.memory)
	s.mus.mus.Bind(c.Runtime.memory)
	s.mus.model.Bind(c.Runtime.memory)
	s.conflict.solver = s
	s.mus.solver = s
	s.mus.isActive = false
	s.mus.maxNumRestarts = ^uint32(0)
	s.mus.constructed = true
	s.propagation.inconsistent = false
	// Empty clause headers and num_frozen precede watches/columns; the actual
	// m_case_split_queue heap initialization occurs only at its member site.
	if s.Variables.watches, err = newSATSolverWatchLists(c.Budget); err != nil {
		return s, err
	}
	if s.Variables.activity, err = newSATSolverActivity(c.Budget); err != nil {
		return s, err
	}
	s.Variables.activityInc = 128
	if s.Variables.queue, err = newSATSolverVarQueueWithDebug(s.Variables.activity, c.Budget, c.Debug); err != nil {
		return s, err
	}
	s.Variables.qhead = 0
	s.Variables.scopeLevel = 0
	if s.Variables.trail, s.Variables.trailRef, err = newSATSolverTrail(c.Budget); err != nil {
		return s, err
	}
	if s.params, err = c.ParamsContext.NewValue(); err != nil {
		return s, err
	}
	if err = c.ParamsContext.Assign(&s.params, &c.Params); err != nil {
		return s, err
	}
	// Remaining declared vectors are empty until reached by their algorithms.
	// The consequence antecedent map is nevertheless eagerly constructed.
	s.auxLiterals.Bind(c.Runtime.memory)
	if s.antecedents, err = newSATSolverIndexMap(c.Budget); err != nil {
		return s, err
	}
	s.todoAntecedents.Bind(c.Runtime.memory)
	// GC is charged concrete context for the actual already declared scalar
	// fields and invariant binding, not a second set of conflict counters.
	if s.gc, err = newSATSolverGC(s.conflict, s.cleaner); err != nil {
		return s, err
	}
	s.gc.traceTags = c.TraceTags
	if err = s.updateParams(&c.Params); err != nil {
		return s, err
	}
	s.conflict.conflictsSinceGC = 0
	s.conflict.conflicts = 0
	s.nextSimplify = 0
	s.propagation.numCheckpoints = 0
	s.constructed = true
	return s, nil
}
func (s *Solver) identity() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	return nil
}
func (s *Solver) open() error {
	if e := s.identity(); e != nil {
		return e
	}
	if s.closing || !s.constructed || (s.mus.frames != nil && !s.mus.isActive) || (s.consequenceFrames != nil && !s.consequenceRunning) || (s.mutexFrames != nil && !s.mutexRunning) {
		return arena.ErrClosed
	}
	if e := s.runtime.open(); e != nil {
		return e
	}
	return s.Lifecycle.open()
}
func (s *Solver) updateParams(p *uparams.ParamsRef) error {
	// This helper is also the actual constructor body; no fabricated completed
	// solver gate precedes its reached parameter actions.
	if e := s.identity(); e != nil {
		return e
	}
	if s.closing {
		return arena.ErrClosed
	}
	if e := s.context.Assign(&s.params, p); e != nil {
		return e
	}
	if e := s.config.Update(p); e != nil {
		return e
	}
	if e := s.simplifier.Update(p); e != nil {
		return e
	}
	if e := s.asymm.Update(p); e != nil {
		return e
	}
	if e := s.probing.updateParams(s.context, p); e != nil {
		return e
	}
	if e := s.scc.Update(p); e != nil {
		return e
	}
	cfg, e := s.config.State()
	if e != nil {
		return e
	}
	s.decision.rng.SetSeed(cfg.RandomSeed)
	return nil
}
func (s *Solver) UpdateParams(p *uparams.ParamsRef) error {
	if e := s.open(); e != nil {
		return e
	}
	return s.updateParams(p)
}

// Close first acknowledges explicitly retained unsafe-exception ownership,
// then executes the real destructor body (only for a completed constructor)
// and declaration-reverse member stages. Stopped release retains its stage and
// every later owner. A failed source ctor has no solver destructor body;
// partial cold contexts are separately owning-aborted, never declared complete.
// Close retains the actual source invariant boundary. Owning abort is a
// separate caller action, never a fabricated successful check_invariant.
func (s *Solver) Close() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if !s.closed && s.closeSourceCause != nil && !s.closeGuardAcknowledged {
		return s.closeSourceCause
	}
	return s.closeMembers()
}

// CleanupStopped acknowledges ONLY a previously observed source destructor
// invariant guard. The immutable cause/history remains visible, and every
// remaining real release still uses its original stopped action cursor.
func (s *Solver) CleanupStopped() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closeSourceCause == nil {
		return ErrSATOwner
	}
	if !s.closeGuardAcknowledged {
		if s.closeStage != 1 {
			return ErrSATOwner
		}
		s.closeGuardAcknowledged = true
		s.closeStage++
	}
	return s.closeMembers()
}
func (s *Solver) closeMembers() error {

	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed {
		return nil
	}
	s.closing = true
	for s.closeStage < 45 {
		var e error
		switch s.closeStage {
		case 0:
			if e = s.cleanupMutexes(); e != nil {
				break
			}
			if e = s.cleanupConsequences(); e != nil {
				break
			}
			if e = s.mus.cleanup(); e != nil {
				break
			}
			if s.Lifecycle != nil {
				e = s.Lifecycle.cleanupOwningContinuations()
			}
		case 1:
			if s.constructed && satStorageSourceDebug {
				e = s.Lifecycle.checkInvariant()
				if e != nil {
					s.closeSourceCause = e
				}
			}
		case 2:
			if s.constructed {
				tags := s.traceTags
				if tags == nil {
					tags = diagnostic.BorrowProcessTraceTags()
				}
				e = s.Lifecycle.deleteClausesBody(tags)
			}
		case 3:
			for i := len(s.binaryClauseGraph); i > 0; i-- {
				if e = s.binaryClauseGraph[i-1].Finalize(); e != nil {
					break
				}
			}
			if e == nil {
				s.binaryClauseGraph = nil
			}
		case 4:
			if s.todoAntecedents.self != nil {
				e = s.todoAntecedents.Finalize()
			}
		case 5:
			if s.antecedents != nil {
				e = s.antecedents.Close()
			}
		case 6:
			s.userBinClauses = nil
		case 7:
			if s.auxLiterals.self != nil {
				e = s.auxLiterals.Finalize()
			}
		case 8:
			if s.conflict != nil {
				e = s.conflict.userScopeLiterals.Finalize()
			}
		case 9:
			if s.analysis != nil {
				e = s.analysis.minStack.Finalize()
			}
		case 10:
			if s.analysis != nil {
				e = s.analysis.unmark.Finalize()
			}
		case 11:
			if s.propagation != nil {
				s.propagation.diffLevels.finalize()
			}
		case 12:
			if s.analysis != nil {
				e = s.analysis.extAntecedents.Finalize()
			}
		case 13:
			if s.analysis != nil {
				e = s.analysis.lemma.Finalize()
			}
		case 14:
			if s.conflict != nil {
				e = s.conflict.minCore.Finalize()
			}
		case 15:
			if s.conflict != nil {
				e = s.conflict.core.Finalize()
			}
		case 16:
			if s.conflict != nil {
				s.conflict.assumptionSet.Finalize()
			}
		case 17:
			if s.conflict != nil {
				e = s.conflict.assumptions.Finalize()
			}
		case 18:
			if s.params.Initialized() {
				e = s.context.Del(&s.params)
			}
		case 19:
			// The Go clock profile owns its running measurement; C++'s stopwatch
			// destructor has no source callback. Stop is an explicit clock adapter.
			if s.gc != nil {
				e = s.gc.watch.Stop()
			}
		case 20:
			if s.Lifecycle != nil {
				e = s.Lifecycle.finalizeScopesReinit()
			}
		case 21:
			if s.Variables != nil {
				e = s.Variables.finalizeTrail()
			}
		case 22:
			if s.Variables != nil {
				e = s.Variables.finalizeQueue()

			}
		case 23:
			if s.Variables != nil {
				e = s.Variables.finalizeColumnsWatches()
			}
		case 24:
			if s.Lifecycle != nil {
				e = s.Lifecycle.finalizeClauseVectors()
			}
		case 25:
			if s.mus.constructed {
				if e = s.mus.model.Finalize(); e == nil {
					e = s.mus.mus.Finalize()
				}
				if e == nil {
					e = s.mus.core.Finalize()
				}
			}
		case 26:
			if s.probing != nil {
				e = s.probing.Close()
			}
		case 27:
			if s.asymm != nil {
				e = s.asymm.Close()
			}
		case 28:
			if s.scc != nil {
				e = s.scc.Close()
			}
		case 29:
			if s.simplifier != nil {
				e = s.simplifier.Close()
			}
		case 30:
			if s.converter != nil {
				e = s.converter.Close()
			}
		case 31:
			if s.model.self != nil {
				e = s.model.Finalize()
			}
		case 32:
			if s.cleaner != nil {
				e = s.cleaner.Close()
			}
		case 33:
			if s.Clauses != nil {
				e = s.Clauses.Close()
			}
		case 34:
			if s.decision != nil {
				e = s.decision.Close()
			}
		case 35:
			if s.config != nil {
				e = s.config.Close()
			}
		// Only actual Go cold-context headers remain after the source member
		// actions above. Their idempotent Close acknowledges already drained
		// fields and does not replay source body or allocation events.
		case 36:
			if s.gc != nil {
				e = s.gc.Close()
			}
		case 37:
			if s.conflict != nil {
				e = s.conflict.Close()
			}
		case 38:
			if s.dynamic != nil {
				e = s.dynamic.Close()
			}
		case 39:
			if s.analysis != nil {
				e = s.analysis.Close()
			}
		case 40:
			if s.propagation != nil {
				e = s.propagation.Close()
			}
		case 41:
			if s.Lifecycle != nil {
				e = s.Lifecycle.closeContext()
			}
		case 42:
			if s.Variables != nil {
				e = s.Variables.Close()
			}
		case 43:
			if s.elimTodo != nil {
				e = s.elimTodo.Close()
			}
		case 44:
			s.runtime.liveSolvers--
			s.budget.Release(s.ownerBytes)
			s.ownerBytes = 0
			s.closed = true
		}
		if e != nil {
			return e
		}
		s.closeStage++
	}
	return nil
}
