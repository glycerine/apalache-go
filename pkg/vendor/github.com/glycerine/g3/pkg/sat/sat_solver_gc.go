package sat

// Source: src/sat/sat_solver.cpp:1169–1190,1347–1670; src/sat/sat_solver.h.

import (
	"errors"
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/luby"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

var ErrSATGCUninitialized = errors.New("sat: source GC field is indeterminate before initialization")

// Source policy members bound to real clause/conflict/phase owners. This is not
// full solver construction or search. The sole conflict/phase counters remain in
// the borrowed conflict owner. Pointer compaction continuations are cold charged
// metadata, distinguishing current owned ranges from retained scalar borrows.
type satSolverGC struct {
	self                                                                                     *satSolverGC
	conflict                                                                                 *satSolverConflictResolution
	cleaner                                                                                  *satCleaner
	traceTags                                                                                *diagnostic.TraceTags
	ownerBytes                                                                               uint64
	gcThreshold, restartThreshold, lubyIndex, restarts, restartStat, numFrozen, gcClauseStat uint32
	minDTk                                                                                   float64
	watch                                                                                    stopwatch.Stopwatch
	initialized, closed, closing, running, interrupted, debugInvariant                       bool
	cursor, out, end, drainPrefix, drainSuffix                                               uint32
	scratch                                                                                  []StoredClauseRef
	scratchBytes                                                                             uint64
	sorting, sortSrcScratch                                                                  bool
	sortLeft, sortMid, sortRight, sortEnd, sortOut, sortN                                    uint32
	sortDrainStage, sortDrainIndex                                                           uint32
}

func newSATSolverGC(s *satSolverConflictResolution, c *satCleaner) (*satSolverGC, error) {
	if e := s.open(); e != nil {
		return nil, e
	}
	if e := c.open(); e != nil {
		return nil, e
	}
	if c.lifecycle != s.dynamic.probing.lifecycle {
		return nil, ErrSATOwner
	}
	n := uint64(reflect.TypeOf(satSolverGC{}).Size())
	if e := s.analysis.budget.Reserve(n); e != nil {
		return nil, e
	}
	g := &satSolverGC{conflict: s, cleaner: c, ownerBytes: n}
	g.self = g
	if e := g.lifecycle().bindIntegrity(g); e != nil {
		s.analysis.budget.Release(n)
		return nil, e
	}
	return g, nil
}
func (g *satSolverGC) lifecycle() *satSolverClauseLifecycle {
	return g.conflict.dynamic.probing.lifecycle
}
func (g *satSolverGC) budget() *arena.Budget { return g.conflict.analysis.budget }
func (g *satSolverGC) open() error {
	if g == nil || g.self != g {
		return ErrSATOwner
	}
	if g.closed || g.closing || g.interrupted || g.sorting {
		return arena.ErrClosed
	}
	if e := g.conflict.open(); e != nil {
		return e
	}
	return g.cleaner.open()
}
func (g *satSolverGC) invariant() error {
	if satStorageSourceDebug && g.debugInvariant {
		return g.lifecycle().checkInvariant()
	}
	return nil
}
func (g *satSolverGC) initSearch() error {
	if e := g.open(); e != nil {
		return e
	}
	s := g.conflict
	p := s.analysis.propagation
	p.variables.modelIsCurrent = false
	s.phaseCounter = 0
	s.phaseCacheOn = false
	s.conflictsSinceRestart = 0
	cfg, e := p.config.State()
	if e != nil {
		return e
	}
	g.restartThreshold = cfg.RestartInitial
	g.lubyIndex = 1
	g.gcThreshold = cfg.GCInitial
	g.restarts = 0
	g.minDTk = 1
	g.watch.Reset()
	if e = g.watch.Start(); e != nil {
		return e
	}
	if e = s.core.Shrink(0); e != nil {
		return e
	}
	s.minCoreValid = false
	if e = s.minCore.Shrink(0); e != nil {
		return e
	}
	g.initialized = true
	return nil
}

// Literal source floating conversion: truncate first; out-of-range/NaN source
// casts are undefined and guarded only at that actual conversion site.
func satGCUnsigned(x float64) (uint32, error) {
	x = math.Trunc(x)
	if math.IsNaN(x) || x < 0 || x > float64(^uint32(0)) {
		return 0, arena.ErrBounds
	}
	return uint32(x), nil
}

func (g *satSolverGC) restart() error {
	if e := g.open(); e != nil {
		return e
	}
	if !g.initialized {
		return ErrSATGCUninitialized
	}
	g.restartStat++
	g.restarts++
	if e := g.emitRestartReport(); e != nil {
		return e
	}
	s := g.conflict
	if e := s.popReinit(s.analysis.propagation.variables.scopeLevel); e != nil {
		return e
	}
	s.conflictsSinceRestart = 0
	cfg, e := s.analysis.propagation.config.State()
	if e != nil {
		return e
	}
	switch cfg.Restart {
	case satRestartGeometric:
		v, e := satGCUnsigned(float64(g.restartThreshold) * cfg.RestartFactor)
		if e != nil {
			return e
		}
		g.restartThreshold = v
	case satRestartLuby:
		g.lubyIndex++
		v, e := luby.Value(g.lubyIndex)
		if e != nil {
			return e
		}
		g.restartThreshold = cfg.RestartInitial * v
	default:
		return ErrSATAssertion
	}
	return g.invariant()
}
func (g *satSolverGC) gc() error {
	if e := g.open(); e != nil {
		return e
	}
	if !g.initialized {
		return ErrSATGCUninitialized
	}
	s := g.conflict
	if s.conflictsSinceGC <= g.gcThreshold {
		return nil
	}
	if e := g.invariant(); e != nil {
		return e
	}
	cfg, e := s.analysis.propagation.config.State()
	if e != nil {
		return e
	}
	switch cfg.GCStrategy {
	case satGCGlue, satGCPSM, satGCGluePSM, satGCPSMGlue:
		if cfg.GCStrategy != satGCGlue {
			if e = g.savePSM(); e != nil {
				return e
			}
		}
		if e = g.sortLearned(cfg.GCStrategy); e != nil {
			return e
		}
		if e = g.gcHalf(); e != nil {
			return e
		}
	case satGCDynPSM:
		if s.analysis.propagation.variables.scopeLevel != 0 {
			return nil
		}
		if e = g.gcDynPSM(); e != nil {
			return e
		}
	default:
		return ErrSATAssertion
	}
	s.conflictsSinceGC = 0
	g.gcThreshold += cfg.GCIncrement
	return g.invariant()
}
func (g *satSolverGC) psm(r StoredClauseRef) (uint32, error) {
	p := g.conflict.analysis.propagation
	n, e := p.clauses.Size(r)
	if e != nil {
		return 0, e
	}
	result := uint32(0)
	for i := uint32(0); i < n; i++ {
		l, e := p.clauses.Literal(r, i)
		if e != nil {
			return result, e
		}
		phase, e := p.variables.phaseAt(l.variable())
		if e != nil {
			return result, e
		}
		if l.sign() {
			if phase == 1 {
				result++
			}
		} else if phase == 0 {
			result++
		}
	}
	return result, nil
}
func (g *satSolverGC) savePSM() error {
	a := g.lifecycle().propagation.clauses
	for _, r := range g.lifecycle().learned {
		v, e := g.psm(r)
		if e != nil {
			return e
		}
		if e = a.SetPSM(r, v); e != nil {
			return e
		}
	}
	return nil
}
func (g *satSolverGC) canDelete(r StoredClauseRef) (bool, error) {
	p := g.lifecycle().propagation
	a := p.clauses
	b, e := a.OnReinitStack(r)
	if e != nil {
		return false, e
	}
	if b {
		return false, nil
	}
	n, e := a.Size(r)
	if e != nil {
		return false, e
	}
	if n == 3 {
		return true, nil
	}
	l, e := a.Literal(r, 0)
	if e != nil {
		return false, e
	}
	v, e := p.variables.valueLiteral(l)
	if e != nil {
		return false, e
	}
	if v != lbool.True {
		return true, nil
	}
	j, e := p.variables.justification.read(uint32(l.variable()))
	if e != nil {
		return false, e
	}
	if !j.Clause() {
		return true, nil
	}
	other, e := a.GetClause(j.ClauseOffset())
	return other != r, e
}
func (g *satSolverGC) beginCompaction(start uint32) error {
	l := g.lifecycle()
	if l.cleanerContinuation != nil || l.gcContinuation != nil {
		return ErrSATOwner
	}
	g.cursor = start
	g.out = start
	g.end = uint32(len(l.learned))
	g.drainPrefix = 0
	g.drainSuffix = 0
	g.interrupted = true
	g.running = true
	l.gcContinuation = g
	return nil
}
func (g *satSolverGC) finishCompaction() {
	l := g.lifecycle()
	l.learned = l.learned[:g.out]
	g.interrupted = false
	l.gcContinuation = nil
}
func (g *satSolverGC) gcHalf() error {
	g.traceGC("gc_half", 1494)
	l := g.lifecycle()
	n := uint32(len(l.learned))
	half := n / 2
	if e := g.beginCompaction(half); e != nil {
		return e
	}
	defer func() { g.running = false }()
	for i := half; i < n; i++ {
		r := l.learned[i]
		yes, e := g.canDelete(r)
		if e != nil {
			return e
		}
		if yes {
			if e = l.detachClause(r); e != nil {
				return e
			}
			if e = l.delClause(r); e != nil {
				return e
			}
			g.cursor = i + 1
		} else {
			l.learned[g.out] = r
			g.out++
			g.cursor = i + 1
		}
	}
	g.gcClauseStat += n - g.out
	g.finishCompaction()
	return g.emitHalfReport(l.propagation.config.state.GCStrategy, n-g.out)
}
func (g *satSolverGC) Close() error {
	if g == nil || g.self != g {
		return ErrSATOwner
	}
	if g.closed {
		return nil
	}
	g.closing = true
	if e := g.cleanupInterrupted(); e != nil {
		return e
	}
	if e := g.watch.Stop(); e != nil {
		return e
	}
	g.budget().Release(g.ownerBytes)
	g.ownerBytes = 0
	g.closed = true
	return nil
}
