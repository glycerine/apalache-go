package sat

// Source: src/sat/sat_simplifier.h; src/sat/sat_simplifier.cpp:29–205,1482–1522.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"

	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

// Concrete source member owner. Solver services are real borrowed owners; this
// is not the complete solver constructor. Only kernel owns visited/counters.
type satSimplifier struct {
	debug                                                                        *diagnostic.DebugContext
	scopedStage                                                                  uint32
	subReportActive                                                              bool
	self                                                                         *satSimplifier
	blocked                                                                      satSimplifierBlocked
	elimination                                                                  satSimplifierElimination
	lifecycle                                                                    *satSolverClauseLifecycle
	cleaner                                                                      *satCleaner
	decision                                                                     *satSolverDecision
	useList                                                                      *satClauseUseLists
	kernel                                                                       *satSimplifierKernel
	subTodo                                                                      *satClauseSet
	subBinTodo                                                                   *satBinClauseVector
	dummy                                                                        *satTmpClause
	bsCS                                                                         *satClauseVector
	literalStore                                                                 *satSolverLiteralStore
	bsLS, newCls                                                                 satSolverLiteralListRef
	posCls, negCls                                                               *satClauseWrapperVector
	numCalls, lastSubTrailSize                                                   uint32
	needCleanup, learnedInUseLists                                               bool
	subsumption, resolution, elimVars, elimBlocked                               bool
	blockedAt, blockedLimit, subsumptionLimit, resLimit, occCutoff               uint32
	occRanges, litRanges                                                         [3]uint32
	clsCutoffs                                                                   [2]uint32
	numBlocked, numSubsumed, numElimVars, numSubRes, numElimLits, oldNumElimVars uint32
	subReportSubsumed, subReportSubRes, blockedReportBlocked, elimReportVars     uint32
	subReport, blockedReport, elimReport                                         stopwatch.Stopwatch
	ownerBytes                                                                   uint64
	closeStage                                                                   uint32
	constructed                                                                  bool
	destructorFinalized                                                          bool
	finalizeStage                                                                uint32
	running, closing, closed, scopeActive, debugInvariant                        bool
	bodyCause, cleanupCause                                                      error
	// Actual stopped owning ranges belong here, never inferred from allocator
	// occupancy. Their concrete cursor semantics are implemented by cleanup body.
	cleanupStage, cleanupIndex, cleanupOut, cleanupEnd, drainPrefix, drainSuffix uint32
	cleanupLearned, cleanupInterrupted                                           bool
}

func newSATSimplifier(l *satSolverClauseLifecycle, c *satCleaner, d *satSolverDecision, p *uparams.ParamsRef) (s *satSimplifier, err error) {
	if e := l.constructionBorrow(); e != nil {
		return nil, e
	}
	if e := c.constructionBorrow(); e != nil {
		return nil, e
	}
	if e := d.constructionBorrow(); e != nil {
		return nil, e
	}
	if d.conflict.dynamic.probing.lifecycle != l {
		return nil, ErrSATOwner
	}
	if c.lifecycle != l {
		return nil, ErrSATOwner
	}
	n := uint64(reflect.TypeOf(satSimplifier{}).Size())
	if e := l.budget.Reserve(n); e != nil {
		return nil, e
	}
	s = &satSimplifier{lifecycle: l, cleaner: c, decision: d, ownerBytes: n}
	s.self = s
	// Retain actual owner on a stopped checked constructor-unwind, rather than
	// losing any already constructed member's physical bytes.
	defer func() {
		if err != nil {
			if e := s.Close(); e != nil {
				err = errors.Join(err, e)
			} else {
				s = nil
			}
		}
	}()
	cfg := arena.VectorConfig{Descriptors: arena.Config{Budget: l.budget}}
	if s.useList, err = newSATClauseUseLists(l.propagation.clauses, l.budget, cfg); err != nil {
		return s, err
	}
	if s.subTodo, err = NewSATClauseSet(l.propagation.clauses, l.budget); err != nil {
		return s, err
	}
	if s.subBinTodo, err = NewSATBinClauseVector(l.budget); err != nil {
		return s, err
	}
	// Elimination todo is the same actual set bound by mk_var, not a second set.
	if s.dummy, err = NewSATTmpClause(l.budget); err != nil {
		return s, err
	}
	if s.kernel, err = newSATSimplifierKernel(l.propagation.clauses, s.useList, l.budget); err != nil {
		return s, err
	}
	s.kernel.lifecycle = l
	if err = s.kernel.bindDummy(s.dummy); err != nil {
		return s, err
	}
	if s.bsCS, err = NewSATClauseVector(l.propagation.clauses, l.budget); err != nil {
		return s, err
	}
	if s.literalStore, err = newSATSolverLiteralStore(l.budget); err != nil {
		return s, err
	}
	if s.bsLS, err = s.literalStore.NewList(); err != nil {
		return s, err
	}
	if s.posCls, err = NewSATClauseWrapperVector(l.propagation.clauses, l.budget); err != nil {
		return s, err
	}
	if s.negCls, err = NewSATClauseWrapperVector(l.propagation.clauses, l.budget); err != nil {
		return s, err
	}
	if s.newCls, err = s.literalStore.NewList(); err != nil {
		return s, err
	}
	if err = s.Update(p); err != nil {
		return s, err
	}
	s.resetStatistics()
	s.constructed = true
	return s, nil
}
func (s *satSimplifier) constructionBorrow() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed || s.closing || s.scopeActive || s.cleanupInterrupted || s.blocked.reportActive || s.elimination.reportActive || s.subReportActive || s.finalizeStage != 0 {
		return arena.ErrClosed
	}
	if e := s.lifecycle.constructionBorrow(); e != nil {
		return e
	}
	return s.decision.constructionBorrow()
}
func (s *satSimplifier) open() error {
	if e := s.constructionBorrow(); e != nil {
		return e
	}
	if !s.constructed {
		return arena.ErrClosed
	}
	return s.decision.open()
}
func (s *satSimplifier) invariant() error {
	if satStorageSourceDebug && s.debugInvariant {
		return s.lifecycle.checkInvariant()
	}
	return nil
}
func (s *satSimplifier) Update(p *uparams.ParamsRef) (err error) {
	if e := s.constructionBorrow(); e != nil {
		return e
	}
	cfg := s.lifecycle.propagation.config
	g, e := cfg.moduleParams()
	if e != nil {
		return e
	}
	defer func() {
		if e := cfg.context.Del(&g); err == nil {
			err = e
		}
	}()
	bools := [4]struct {
		name string
		dst  *bool
		def  bool
	}{{"elim_blocked_clauses", &s.elimBlocked, false}, {"resolution", &s.resolution, true}, {"subsumption", &s.subsumption, true}, {"elim_vars", &s.elimVars, true}}
	// Literal source update order interleaves the first two booleans with numeric
	// fields, so each read/publish follows the source below, not field grouping.
	readBool := func(i int) error {
		x := bools[i]
		v, e := cfg.context.GetBoolFallbackName(p, &x.name, &g, x.def)
		if e == nil {
			*x.dst = v
		}
		return e
	}
	readUint := func(name string, dst *uint32, def uint32) error {
		v, e := cfg.context.GetUIntFallbackName(p, &name, &g, def)
		if e == nil {
			*dst = v
		}
		return e
	}
	if e = readBool(0); e != nil {
		return e
	}
	if e = readUint("elim_blocked_clauses_at", &s.blockedAt, 2); e != nil {
		return e
	}
	if e = readUint("blocked_clause_limit", &s.blockedLimit, 100000000); e != nil {
		return e
	}
	if e = readBool(1); e != nil {
		return e
	}
	if e = readUint("resolution.limit", &s.resLimit, 500000000); e != nil {
		return e
	}
	if e = readUint("resolution.occ_cutoff", &s.occCutoff, 10); e != nil {
		return e
	}
	for i, x := range [3]struct {
		name string
		def  uint32
	}{{"resolution.occ_cutoff_range1", 8}, {"resolution.occ_cutoff_range2", 5}, {"resolution.occ_cutoff_range3", 3}} {
		if e = readUint(x.name, &s.occRanges[i], x.def); e != nil {
			return e
		}
	}
	for i, x := range [3]struct {
		name string
		def  uint32
	}{{"resolution.lit_cutoff_range1", 700}, {"resolution.lit_cutoff_range2", 400}, {"resolution.lit_cutoff_range3", 300}} {
		if e = readUint(x.name, &s.litRanges[i], x.def); e != nil {
			return e
		}
	}
	if e = readUint("resolution.cls_cutoff1", &s.clsCutoffs[0], 100000000); e != nil {
		return e
	}
	if e = readUint("resolution.cls_cutoff2", &s.clsCutoffs[1], 700000000); e != nil {
		return e
	}
	if e = readBool(2); e != nil {
		return e
	}
	if e = readUint("subsumption.limit", &s.subsumptionLimit, 100000000); e != nil {
		return e
	}
	return readBool(3)
}
func (s *satSimplifier) resetStatistics() {
	s.numBlocked = 0
	s.numSubsumed = 0
	s.numElimVars = 0
	s.numSubRes = 0
	s.numElimLits = 0
}

func (s *satSimplifier) Run(learned bool) (err error) {
	if e := s.open(); e != nil {
		return e
	}
	p := s.lifecycle.propagation
	if p.inconsistent {
		return nil
	}
	if !s.subsumption && !s.elimBlocked && !s.resolution {
		return nil
	}
	if e := s.initialize(); e != nil {
		return e
	}
	if e := s.invariant(); e != nil {
		return e
	}
	s.traceRunSolverDisplay("before_simplifier", "operator()", 162)
	if e := s.subTodo.Reset(); e != nil {
		return e
	}
	if e := s.subBinTodo.Reset(); e != nil {
		return e
	}
	if _, e := s.cleaner.run(true); e != nil {
		return e
	}
	var e error
	s.lastSubTrailSize, e = p.variables.trail.Size(p.variables.trailRef)
	if e != nil {
		return e
	}
	s.traceRunSolverDisplay("after_cleanup", "operator()", 168)
	if e = s.invariant(); e != nil {
		return e
	}
	s.needCleanup = false
	n, e := p.variables.numVarsChecked()
	if e != nil {
		return e
	}
	if e = s.useList.Init(n); e != nil {
		return e
	}
	s.learnedInUseLists = false
	if learned {
		if e = s.registerClauses(true); e != nil {
			return e
		}
		s.learnedInUseLists = true
	}
	if e = s.registerClauses(false); e != nil {
		return e
	}
	if !learned && (s.elimBlocked || s.blockedAt == s.numCalls) {
		if e = s.eliminateBlocked(); e != nil {
			return e
		}
	}
	if !learned {
		s.numCalls++
	}
	s.kernel.subCounter = int32(s.subsumptionLimit)
	s.kernel.elimCounter = int32(s.resLimit)
	s.oldNumElimVars = s.numElimVars
	s.scopeActive = true
	s.scopedStage = 0
	s.lifecycle.simplifierContinuation = s
	s.running = true
	defer func() {
		s.running = false
		s.bodyCause = err
		// Older scoped-finalize destructor cannot overtake a stopped inner
		// lexical destructor/report; owning Close first acknowledges that phase.
		if s.elimination.reportActive || s.subReportActive || s.blocked.reportActive {
			return
		}
		if e := s.scopedFinalize(); e != nil {
			s.cleanupCause = e
			if err == nil {
				err = e
			} else {
				err = errors.Join(err, e)
			}
		}
	}()
	for {
		if s.subsumption {
			if e = s.subsume(); e != nil {
				return e
			}
		}
		if p.inconsistent {
			return nil
		}
		if !learned && s.resolution {
			if e = s.eliminateVariables(); e != nil {
				return e
			}
		}
		if p.inconsistent {
			return nil
		}
		if !s.subsumption || s.kernel.subCounter < 0 || s.subTodo.Empty() {
			break
		}
	}
	return nil
}
func (s *satSimplifier) initialize() error {
	s.needCleanup = false
	if _, e := s.cleaner.run(true); e != nil {
		return e
	}
	var e error
	p := s.lifecycle.propagation
	s.lastSubTrailSize, e = p.variables.trail.Size(p.variables.trailRef)
	if e != nil {
		return e
	}
	n, e := p.variables.numVarsChecked()
	if e != nil {
		return e
	}
	if e = s.useList.Init(n); e != nil {
		return e
	}
	if e = s.subTodo.Reset(); e != nil {
		return e
	}
	if e = s.subBinTodo.Reset(); e != nil {
		return e
	}
	if e = p.variables.elimTodo.Reset(); e != nil {
		return e
	}
	n, e = p.variables.numVarsChecked()
	if e != nil {
		return e
	}
	if e = s.kernel.InitVisited(n); e != nil {
		return e
	}
	s.traceRunSolverDisplay("after_cleanup", "initialize", 147)
	return s.invariant()
}

// Source explicit finalize order differs from reverse member destruction.
func (s *satSimplifier) Finalize() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	// Source action order; the cursor is an explicit owning continuation only
	// for interrupted checked destruction, never a source exception retry claim.
	for s.finalizeStage < 7 {
		var e error
		switch s.finalizeStage {
		case 0:
			e = s.useList.Finalize()
		case 1:
			e = s.subTodo.Finalize()
		case 2:
			e = s.subBinTodo.Finalize()
		case 3:
			e = s.lifecycle.propagation.variables.elimTodo.Finalize()
		case 4:
			e = s.kernel.Finalize()
		case 5:
			e = s.bsCS.Finalize()
		case 6:
			if s.bsLS != (satSolverLiteralListRef{}) {
				e = s.literalStore.Free(s.bsLS)
				if e == nil {
					s.bsLS = satSolverLiteralListRef{}
				}
			}
		}
		if e != nil {
			return e
		}
		s.finalizeStage++
	}
	s.finalizeStage = 0
	return nil
}
func (s *satSimplifier) Close() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed {
		return nil
	}
	s.closing = true
	if e := s.closeSubsumeReport(); e != nil {
		return e
	}
	if e := s.closeElimination(); e != nil {
		return e
	}
	if e := s.closeBlocked(); e != nil {
		return e
	}
	if s.cleanupInterrupted {
		if e := s.cleanupStopped(); e != nil {
			return e
		}
	}
	if s.scopeActive {
		if e := s.scopedFinalize(); e != nil {
			return e
		}
	}

	if s.constructed && !s.destructorFinalized {
		if e := s.Finalize(); e != nil {
			return e
		}
		s.destructorFinalized = true
	}
	// Reverse actual constructed member order. A stopped release retains this
	// owner and all later members; no ignored defer or registry destruction.
	for s.closeStage < 10 {
		var e error
		switch s.closeStage {
		case 0:
			if s.literalStore != nil && s.newCls != (satSolverLiteralListRef{}) {
				e = s.literalStore.Free(s.newCls)
				if e == nil {
					s.newCls = satSolverLiteralListRef{}
				}
			}
		case 1:
			if s.negCls != nil {
				e = s.negCls.Close()
			}
		case 2:
			if s.posCls != nil {
				e = s.posCls.Close()
			}
		case 3:
			if s.literalStore != nil {
				e = s.literalStore.Close()
			}
		case 4:
			if s.bsCS != nil {
				e = s.bsCS.Close()
			}
		case 5:
			if s.kernel != nil {
				e = s.kernel.Close()
			}
		case 6:
			if s.dummy != nil {
				e = s.dummy.Close()
			}
		case 7:
			if s.subBinTodo != nil {
				e = s.subBinTodo.Close()
			}
		case 8:
			if s.subTodo != nil {
				e = s.subTodo.Close()
			}
		case 9:
			if s.useList != nil {
				e = s.useList.Close()
			}
		}
		if e != nil {
			return e
		}
		s.closeStage++
	}
	s.lifecycle.budget.Release(s.ownerBytes)
	s.ownerBytes = 0
	s.closed = true
	return nil
}
