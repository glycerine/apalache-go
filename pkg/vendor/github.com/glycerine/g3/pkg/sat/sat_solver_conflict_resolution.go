package sat

// Source: src/sat/sat_solver.cpp:1675–2219; src/sat/sat_solver.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

var ErrSATMissingParallelBinding = errors.New("g3: missing source SAT parallel-exchange binding")
var ErrSATMissingMUSBinding = errors.New("g3: missing source SAT MUS/model binding")

// Real conflict member grouping, not a solver constructor or Check substitute.
// Borrowed analysis/dynamic/probing/lifecycle/Runtime must outlive this owner.
// Source search initialization of the phase/restart fields remains a separate
// entry action; the constructor supplies its documented explicit initial state.
type satSolverConflictResolution struct {
	solver                                                                          *Solver // actual optional full-owner MUS binding, never a successful callback
	self                                                                            *satSolverConflictResolution
	analysis                                                                        *satSolverConflictAnalysis
	dynamic                                                                         *satSolverDynamicSubsumption
	traceTags                                                                       *diagnostic.TraceTags
	assumptions, userScopeLiterals, core, minCore                                   satUintColumn
	assumptionSet                                                                   satLiteralSetStorage
	conflictLevel, conflictStat, conflicts, conflictsSinceRestart, conflictsSinceGC uint32
	phaseCounter                                                                    uint32
	phaseCacheOn, minCoreValid, parallelPresent                                     bool
	ownerBytes                                                                      uint64
	closing, closed                                                                 bool
}

func newSATSolverConflictResolution(a *satSolverConflictAnalysis, d *satSolverDynamicSubsumption) (*satSolverConflictResolution, error) {
	if e := a.constructionBorrow(); e != nil {
		return nil, e
	}
	if e := d.constructionBorrow(); e != nil {
		return nil, e
	}
	if d.analysis != a {
		return nil, ErrSATOwner
	}
	n := uint64(reflect.TypeOf(satSolverConflictResolution{}).Size())
	if e := a.budget.Reserve(n); e != nil {
		return nil, e
	}
	s := &satSolverConflictResolution{analysis: a, dynamic: d, ownerBytes: n}
	s.self = s
	for _, c := range []*satUintColumn{&s.assumptions, &s.userScopeLiterals, &s.core, &s.minCore} {
		c.Bind(a.propagation.memory)
	}
	return s, nil
}
func (s *satSolverConflictResolution) constructionBorrow() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed || s.closing {
		return arena.ErrClosed
	}
	if e := s.analysis.constructionBorrow(); e != nil {
		return e
	}
	return s.dynamic.constructionBorrow()
}
func (s *satSolverConflictResolution) open() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed || s.closing {
		return arena.ErrClosed
	}
	if e := s.analysis.open(); e != nil {
		return e
	}
	return s.dynamic.open()
}
func (s *satSolverConflictResolution) Close() error {
	if s == nil || s.self != s {
		return ErrSATOwner
	}
	if s.closed {
		return nil
	}
	s.closing = true
	// Source reverse relevant declaration order: user-scope literals, min_core,
	// core, assumption_set, assumptions (sat_solver.h:130–132,344,443).
	for _, c := range []*satUintColumn{&s.userScopeLiterals, &s.minCore, &s.core} {
		if e := c.Finalize(); e != nil {
			return e
		}
	}
	s.assumptionSet.Finalize()
	for _, c := range []*satUintColumn{&s.assumptions} {
		if e := c.Finalize(); e != nil {
			return e
		}
	}
	s.analysis.budget.Release(s.ownerBytes)
	s.ownerBytes = 0
	s.closed = true
	return nil
}
func (s *satSolverConflictResolution) resolveConflict() (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	for {
		ok, e := s.resolveConflictCore()
		if e != nil {
			return false, e
		}
		if satStorageSourceDebug {
			if e = s.assertNoMarks(); e != nil {
				return false, e
			}
		}
		if !ok {
			return false, nil
		}
		if !s.analysis.propagation.inconsistent {
			return true, nil
		}
	}
}
func (s *satSolverConflictResolution) resolveConflictCore() (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	a := s.analysis
	p := a.propagation
	v := p.variables
	l := s.dynamic.probing.lifecycle
	s.conflictStat++
	s.conflicts++
	s.conflictsSinceRestart++
	s.conflictsSinceGC++
	level, e := s.getMaxLevel(p.notLiteral, p.conflict)
	if e != nil {
		return false, e
	}
	s.conflictLevel = level
	s.traceConflictDetected(p.notLiteral)
	if level <= 1 {
		tracking, e := s.trackingAssumptions()
		if e != nil {
			return false, e
		}
		if tracking {
			return false, s.resolveConflictForUnsatCore()
		}
	}
	if level == 0 {
		return false, nil
	}
	if e = a.lemma.Shrink(0); e != nil {
		return false, e
	}
	if e = s.forgetPhaseOfVars(level); e != nil {
		return false, e
	}
	idx, e := s.skipLiteralsAboveConflictLevel()
	if e != nil {
		return false, e
	}
	if e = a.lemma.Push(uint32(DefaultLiteral())); e != nil {
		return false, e
	}
	marks := uint32(0)
	if p.notLiteral != DefaultLiteral() {
		s.traceConflictNotLiteral(p.notLiteral)
		if e = s.processAntecedent(p.notLiteral, &marks); e != nil {
			return false, e
		}
	}
	consequent, j := p.notLiteral, p.conflict
	for {
		s.traceProcessingConsequent(consequent, marks, j)
		if e = s.processConsequent(consequent, j, 0, &marks); e != nil {
			return false, e
		}
		for {
			lit, e := v.trailRead(idx)
			if e != nil {
				return false, e
			}
			marked, e := a.marked(uint32(lit.variable()))
			if e != nil {
				return false, e
			}
			if marked {
				break
			}
			if satStorageSourceDebug && idx == 0 {
				return false, ErrSATAssertion
			}
			idx--
		}
		consequent, e = v.trailRead(idx)
		if e != nil {
			return false, e
		}
		id := uint32(consequent.variable())
		if satStorageSourceDebug {
			lv, e := v.levelVar(consequent.variable())
			if e != nil {
				return false, e
			}
			if lv != s.conflictLevel {
				return false, ErrSATAssertion
			}
		}
		j, e = v.justification.read(id)
		if e != nil {
			return false, e
		}
		idx--
		marks--
		if e = a.resetMark(id); e != nil {
			return false, e
		}
		if marks == 0 {
			break
		}
	}
	if e = a.lemma.Write(0, uint32(consequent.Negated())); e != nil {
		return false, e
	}
	s.traceNewLemma(false)
	cfg, e := p.config.State()
	if e != nil {
		return false, e
	}
	if cfg.MinimizeLemmas {
		if e = a.minimizeLemma(); e != nil {
			return false, e
		}
		if e = a.resetLemmaVarMarks(); e != nil {
			return false, e
		}
		if cfg.DynSubRes {
			if e = s.dynamic.run(); e != nil {
				return false, e
			}
		}
		s.traceNewLemma(true)
	} else {
		if e = a.resetLemmaVarMarks(); e != nil {
			return false, e
		}
	}
	n, e := a.lemma.Size()
	if e != nil {
		return false, e
	}
	span := satConflictColumnSpan(&a.lemma)
	newLevel := uint32(0)
	for i := uint32(1); i < n; i++ {
		lit, e := span.literal(i)
		if e != nil {
			return false, e
		}
		lv, e := v.levelVar(lit.variable())
		if e != nil {
			return false, e
		}
		if lv > newLevel {
			newLevel = lv
		}
	}
	glue, e := p.numDiffLevelsSpan(n, span)
	if e != nil {
		return false, e
	}
	if e = s.popReinit(v.scopeLevel - newLevel); e != nil {
		return false, e
	}
	s.tracePostBackjumpDisplay()
	lemma, e := l.mkClauseCoreSpan(n, span, true)
	if e != nil {
		return false, e
	}
	if lemma != (StoredClauseRef{}) {
		if e = p.clauses.SetGlue(lemma, glue); e != nil {
			return false, e
		}
	}
	v.decayActivity()
	return true, s.updatePhaseCounters()
}
