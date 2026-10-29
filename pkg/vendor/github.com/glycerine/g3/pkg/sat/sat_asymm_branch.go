package sat

// Source: src/sat/sat_asymm_branch.cpp; src/sat/sat_asymm_branch.h; src/sat/sat_asymm_branch_params.pyg.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"

	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

type satAsymmBranch struct {
	self                                                                                  *satAsymmBranch
	solver                                                                                *Solver
	lifecycle                                                                             *satSolverClauseLifecycle
	ownerBytes                                                                            uint64
	enabled                                                                               bool
	rounds, limit, elimLiterals, reportElimLiterals                                       uint32
	savedPhase                                                                            []int8
	report                                                                                stopwatch.Stopwatch
	reportActive, closing, closed, debugInvariant, running, interrupted                   bool
	cursor, out, end, drainPrefix, drainSuffix                                            uint32
	detachedClause, discardedClause                                                       StoredClauseRef
	detachActive                                                                          bool
	bodyCause, cleanupCause                                                               error
	scratch                                                                               []StoredClauseRef
	scratchBytes                                                                          uint64
	sorting, sortSrcScratch                                                               bool
	sortLeft, sortMid, sortRight, sortEnd, sortOut, sortN, sortDrainStage, sortDrainIndex uint32
}

func newSATAsymmBranch(l *satSolverClauseLifecycle, p *uparams.ParamsRef) (*satAsymmBranch, error) {
	if e := l.constructionBorrow(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(satAsymmBranch{}).Size())
	if e := l.budget.Reserve(n); e != nil {
		return nil, e
	}
	a := &satAsymmBranch{lifecycle: l, ownerBytes: n}
	a.self = a
	l.propagation.asymmCounter = 0
	if e := a.Update(p); e != nil {
		l.budget.Release(n)
		return nil, e
	}
	a.resetStatistics()
	return a, nil
}
func (a *satAsymmBranch) constructionBorrow() error {
	if a == nil || a.self != a {
		return ErrSATOwner
	}
	if a.closed || a.closing || a.reportActive || a.interrupted || a.sorting || a.detachActive || a.savedPhase != nil {
		return arena.ErrClosed
	}
	return a.lifecycle.constructionBorrow()
}
func (a *satAsymmBranch) open() error {
	if a == nil || a.self != a {
		return ErrSATOwner
	}
	if a.closed || a.closing || a.reportActive || a.interrupted || a.sorting || a.detachActive || a.savedPhase != nil {
		return arena.ErrClosed
	}
	return a.lifecycle.open()
}
func (a *satAsymmBranch) Update(p *uparams.ParamsRef) (err error) {
	if e := a.constructionBorrow(); e != nil {
		return e
	}
	cfg := a.lifecycle.propagation.config
	g, e := cfg.moduleParams()
	if e != nil {
		return e
	}
	defer func() {
		if e := cfg.context.Del(&g); err == nil {
			err = e
		}
	}()
	key := "asymm_branch"
	b, e := cfg.context.GetBoolFallbackName(p, &key, &g, true)
	if e != nil {
		return e
	}
	a.enabled = b
	key = "asymm_branch.rounds"
	a.rounds, e = cfg.context.GetUIntFallbackName(p, &key, &g, 32)
	if e != nil {
		return e
	}
	key = "asymm_branch.limit"
	a.limit, e = cfg.context.GetUIntFallbackName(p, &key, &g, 100000000)
	if e != nil {
		return e
	}
	if a.limit > uint32(1<<31-1) {
		a.limit = 1<<31 - 1
	}
	return nil
}
func (a *satAsymmBranch) resetStatistics() { a.elimLiterals = 0 }
func (a *satAsymmBranch) collectStatistics(s *statistics.Statistics, k statistics.Key) error {
	if e := a.open(); e != nil {
		return e
	}
	s.UpdateUint(k, a.elimLiterals)
	return nil
}
func collectAsymmBranchParamDescrs(d *uparams.ParamDescrs) error {
	for _, x := range [3]struct {
		name             string
		kind             uparams.ParamKind
		description, def string
	}{{"asymm_branch", uparams.ParamBool, "asymmetric branching", "true"}, {"asymm_branch.rounds", uparams.ParamUInt, "maximum number of rounds of asymmetric branching", "32"}, {"asymm_branch.limit", uparams.ParamUInt, "approx. maximum number of literals visited during asymmetric branching", "100000000"}} {
		if e := satConfigInsertDescriptor(d, x.name, x.kind, x.description, x.def); e != nil {
			return e
		}
	}
	return nil
}
func (a *satAsymmBranch) dec(n uint32) {
	p := a.lifecycle.propagation
	p.asymmCounter = int32(uint32(p.asymmCounter) - n)
}
func (a *satAsymmBranch) negateCounter() error {
	p := a.lifecycle.propagation
	if p.asymmCounter == -1<<31 {
		return arena.ErrBounds
	}
	p.asymmCounter = -p.asymmCounter
	return nil
}
func (a *satAsymmBranch) invariant() error {
	if satStorageSourceDebug && a.debugInvariant {
		return a.lifecycle.checkInvariant()
	}
	return nil
}
func (a *satAsymmBranch) trailInvariant() error {
	if satStorageSourceDebug {
		p := a.lifecycle.propagation
		n, e := p.variables.trail.Size(p.variables.trailRef)
		if e != nil {
			return e
		}
		if p.variables.qhead != n {
			return ErrSATAssertion
		}
	}
	return nil
}
func (a *satAsymmBranch) run(force bool) (err error) {
	if e := a.open(); e != nil {
		return e
	}
	p := a.lifecycle.propagation
	if !a.enabled {
		return nil
	}
	if _, e := a.lifecycle.propagate(false); e != nil {
		return e
	}
	if p.inconsistent {
		return nil
	}
	if !force && p.asymmCounter > 0 {
		return nil
	}
	if e := a.invariant(); e != nil {
		return e
	}
	a.traceSolverDisplay()
	a.report = stopwatch.Stopwatch{}
	a.reportElimLiterals = a.elimLiterals
	if e := a.report.Start(); e != nil {
		return e
	}
	a.reportActive = true
	a.bodyCause = nil
	a.cleanupCause = nil
	defer func() { err = a.finishRun(err) }()
	a.savedPhase = append([]int8(nil), p.variables.phase...)
	p.asymmCounter = 0
	limit := -int32(a.limit)
	if e := a.sortOriginal(); e != nil {
		return e
	}
	a.dec(uint32(len(a.lifecycle.Original)))
	if e := a.trailInvariant(); e != nil {
		return e
	}
	if e := a.beginCompaction(); e != nil {
		return e
	}
	defer func() { a.running = false }()
	for i := uint32(0); i < a.end; i++ {
		if p.inconsistent {
			a.appendRemainder(i)
			break
		}
		if e := a.trailInvariant(); e != nil {
			return e
		}
		if p.asymmCounter < limit || p.inconsistent {
			a.lifecycle.Original[a.out] = a.lifecycle.Original[i]
			a.out++
			a.cursor = i + 1
			continue
		}
		e := p.checkpoint()
		if e == nil {
			r := a.lifecycle.Original[i]
			var n uint32
			n, e = p.clauses.Size(r)
			if e == nil {
				a.dec(n)
				var keep bool
				keep, e = a.process(r)
				if e == nil {
					if keep {
						a.lifecycle.Original[a.out] = r
						a.out++
					}
					a.cursor = i + 1
				}
			}
		}
		if e != nil {
			var solver SATSolverException
			if errors.As(e, &solver) && !a.detachActive {
				a.appendRemainder(i)
				a.finishCompaction()
				if x := a.negateCounter(); x != nil {
					return errors.Join(e, x)
				}
			}
			return e
		}
	}
	a.finishCompaction()
	if e := a.negateCounter(); e != nil {
		return e
	}
	// Only normal completion restores the saved polarities. Exception paths
	// retain the phase changes made by the interrupted source algorithm.
	p.variables.phase = append(p.variables.phase[:0], a.savedPhase...)
	return a.invariant()
}
func (a *satAsymmBranch) beginCompaction() error {
	l := a.lifecycle
	if l.asymmContinuation != nil {
		return ErrSATOwner
	}
	a.cursor = 0
	a.out = 0
	a.end = uint32(len(l.Original))
	a.drainPrefix = 0
	a.drainSuffix = 0
	a.interrupted = true
	a.running = true
	l.asymmContinuation = a
	return nil
}
func (a *satAsymmBranch) appendRemainder(i uint32) {
	for ; i < a.end; i++ {
		a.lifecycle.Original[a.out] = a.lifecycle.Original[i]
		a.out++
		a.cursor = i + 1
	}
}
func (a *satAsymmBranch) finishCompaction() {
	a.lifecycle.Original = a.lifecycle.Original[:a.out]
	a.interrupted = false
	a.lifecycle.asymmContinuation = nil
}

// Drop the native snapshot before finishing the timed report.
func (a *satAsymmBranch) finishRun(cause error) error {
	a.bodyCause = cause
	a.savedPhase = nil
	if a.reportActive {
		if e := a.report.Stop(); e != nil {
			if cause == nil {
				return e
			}
			return errors.Join(cause, e)
		}
		a.reportActive = false
		if e := a.emitReport(); e != nil {
			if cause == nil {
				return e
			}
			return errors.Join(cause, e)
		}
	}
	return cause
}
func (a *satAsymmBranch) Close() error {
	if a == nil || a.self != a {
		return ErrSATOwner
	}
	if a.closed {
		return nil
	}
	a.closing = true
	if e := a.finishRun(nil); e != nil {
		return e
	}
	if e := a.cleanupInterrupted(); e != nil {
		return e
	}
	if a.discardedClause != (StoredClauseRef{}) {
		if e := a.lifecycle.propagation.clauses.Delete(a.discardedClause); e != nil {
			return e
		}
		a.discardedClause = StoredClauseRef{}
	}

	a.lifecycle.budget.Release(a.ownerBytes)
	a.ownerBytes = 0
	a.solver = nil
	a.closed = true
	return nil
}
