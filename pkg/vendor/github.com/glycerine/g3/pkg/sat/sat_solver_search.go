package sat

// Source: src/sat/sat_solver.cpp:161–176,723–802,993–1118,1191–1255,2751–2756.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var ErrSATMissingDimacsBinding = errors.New("sat: missing source DIMACS printer binding")

type satSolverAbort struct{}

func (satSolverAbort) Error() string { return "sat: abort_solver" }

func (s *Solver) MkVar(external, decision bool) (BoolVar, error) {
	if e := s.open(); e != nil {
		return 0, e
	}
	return s.Variables.mkVar(external, decision)
}
func (s *Solver) MkClause(lits []Literal) error {
	if e := s.open(); e != nil {
		return e
	}
	s.Variables.modelIsCurrent = false
	if satStorageSourceDebug {
		for _, l := range lits {
			elim, e := s.Variables.wasEliminated(l.variable())
			if e != nil {
				return e
			}
			if elim {
				return ErrSATAssertion
			}
		}
	}
	n, e := s.conflict.userScopeLiterals.Size()
	if e != nil {
		return e
	}
	if n == 0 {
		_, e = s.Lifecycle.MkClauseCore(uint32(len(lits)), lits, false)
		return e
	}
	if e = s.auxLiterals.Shrink(0); e != nil {
		return e
	}
	for _, l := range lits {
		if e = s.auxLiterals.Push(uint32(l)); e != nil {
			return e
		}
	}
	for i := uint32(0); i < n; i++ {
		l, e := s.conflict.userScopeLiterals.Read(i)
		if e != nil {
			return e
		}
		if e = s.auxLiterals.Push(l); e != nil {
			return e
		}
	}
	size, e := s.auxLiterals.Size()
	if e != nil {
		return e
	}
	_, e = s.Lifecycle.mkClauseCoreSpan(size, satConflictColumnSpan(&s.auxLiterals), false)
	return e
}
func (s *Solver) resetAssumptions() error {
	if e := s.conflict.assumptions.Shrink(0); e != nil {
		return e
	}
	s.conflict.assumptionSet.Reset()
	return nil
}
func (s *Solver) popToBase() error {
	if e := s.resetAssumptions(); e != nil {
		return e
	}
	return s.Lifecycle.pop(s.Variables.scopeLevel)
}
func (s *Solver) initAssumptions(lits []Literal) error {
	return s.initAssumptionsSpan(uint32(len(lits)), satSolverLiteralSpan{literals: lits})
}
func (s *Solver) initAssumptionsSpan(count uint32, span satSolverLiteralSpan) error {
	n, e := s.conflict.userScopeLiterals.Size()
	if e != nil {
		return e
	}
	if count == 0 && n == 0 {
		return nil
	}
	if e = s.resetAssumptions(); e != nil {
		return e
	}
	if e = s.Lifecycle.push(); e != nil {
		return e
	}
	if _, e = s.Lifecycle.propagate(false); e != nil {
		return e
	}
	if s.propagation.inconsistent {
		return nil
	}
	s.traceInitAssumptions(count, span)
	for i := uint32(0); !s.propagation.inconsistent; i++ {
		n, e := s.conflict.userScopeLiterals.Size()
		if e != nil {
			return e
		}
		if i >= n {
			break
		}
		l, e := s.conflict.userScopeLiterals.Read(i)
		if e != nil {
			return e
		}
		if e = s.propagation.assign(Literal(l).Negated(), satJustification{}); e != nil {
			return e
		}
	}
	for i := uint32(0); i < count && !s.propagation.inconsistent; i++ {
		l, e := span.literal(i)
		if e != nil {
			return e
		}
		if satStorageSourceDebug {
			yes, e := s.Variables.IsExternal(l.variable())
			if e != nil {
				return e
			}
			if !yes {
				return ErrSATAssertion
			}
		}
		if e = s.conflict.assumptionSet.Insert(l); e != nil {
			return e
		}
		if e = s.conflict.assumptions.Push(uint32(l)); e != nil {
			return e
		}
		if e = s.propagation.assign(l, satJustification{}); e != nil {
			return e
		}
	}
	return nil
}
func (s *Solver) checkInconsistent() (bool, error) {
	if !s.propagation.inconsistent {
		return false, nil
	}
	tracking, e := s.conflict.trackingAssumptions()
	if e != nil {
		return false, e
	}
	if tracking {
		if _, e = s.conflict.resolveConflict(); e != nil {
			return false, e
		}
	}
	return true, nil
}
func (s *Solver) simplifyProblem() error {
	if s.conflict.conflicts < s.nextSimplify {
		return nil
	}
	if e := s.emitSimplifyNotice(); e != nil {
		return e
	}
	s.traceSimplifyProblem()
	if e := s.Lifecycle.pop(s.Variables.scopeLevel); e != nil {
		return e
	}
	if satStorageSourceDebug && s.Variables.scopeLevel != 0 {
		return ErrSATAssertion
	}
	if _, e := s.cleaner.run(false); e != nil {
		return e
	}
	if _, e := s.scc.run(); e != nil {
		return e
	}
	if e := s.simplifier.Run(false); e != nil {
		return e
	}
	if len(s.Lifecycle.learned) != 0 {
		if e := s.simplifier.Run(true); e != nil {
			return e
		}
	}
	for _, r := range s.Variables.watches.lists {
		if e := s.Variables.watches.store.SortByKind(r); e != nil {
			return e
		}
	}
	if _, e := s.probing.run(false); e != nil {
		return e
	}
	if e := s.asymm.run(false); e != nil {
		return e
	}
	if s.propagation.extensionPresent {
		return ErrSATMissingSourceExtBinding
	}
	s.traceSimplifyCompleted()
	if e := s.conflict.reinitAssumptions(); e != nil {
		return e
	}
	cfg, e := s.config.State()
	if e != nil {
		return e
	}
	if s.nextSimplify == 0 {
		s.nextSimplify = cfg.RestartInitial * cfg.SimplifyMult1
	} else {
		n, e := satGCUnsigned(float64(s.conflict.conflicts) * cfg.SimplifyMult2)
		if e != nil {
			return e
		}
		s.nextSimplify = n
		if s.nextSimplify > s.conflict.conflicts+cfg.SimplifyMax {
			s.nextSimplify = s.conflict.conflicts + cfg.SimplifyMax
		}
	}
	return nil
}
func (s *Solver) backjumpStep() (lbool.LBool, bool, error) {
	done := true
	if _, e := s.Lifecycle.propagate(true); e != nil {
		return lbool.Undef, done, e
	}
	if !s.propagation.inconsistent {
		return lbool.True, done, nil
	}
	yes, e := s.conflict.resolveConflict()
	if e != nil {
		return lbool.Undef, done, e
	}
	if !yes {
		return lbool.False, done, nil
	}
	cfg, e := s.config.State()
	if e != nil {
		return lbool.Undef, done, e
	}
	if s.conflict.conflicts > cfg.MaxConflicts || s.conflict.conflictsSinceRestart > s.gc.restartThreshold {
		return lbool.Undef, done, nil
	}
	if s.Variables.scopeLevel == 0 {
		if e = s.cleaner.cleanup(); e != nil {
			return lbool.Undef, done, e
		}
		if s.propagation.inconsistent {
			s.traceConflictAtLevelZero()
			return lbool.False, done, nil
		}
		if e = s.gc.gc(); e != nil {
			return lbool.Undef, done, e
		}
	}
	return lbool.True, false, nil
}
func (s *Solver) boundedSearch() (lbool.LBool, error) {
	for {
		if e := s.propagation.checkpoint(); e != nil {
			return lbool.Undef, e
		}
		done := false
		for !done {
			r, d, e := s.backjumpStep()
			if e != nil {
				return lbool.Undef, e
			}
			if r != lbool.True {
				return r, nil
			}
			done = d
		}
		if e := s.gc.gc(); e != nil {
			return lbool.Undef, e
		}
		yes, e := s.decision.decide()
		if e != nil {
			return lbool.Undef, e
		}
		if !yes {
			if s.propagation.extensionPresent {
				return lbool.Undef, ErrSATMissingSourceExtBinding
			}
			if e = s.mkModel(); e != nil {
				return lbool.Undef, e
			}
			return lbool.True, nil
		}
	}
}
func (s *Solver) Check(lits []Literal) (lbool.LBool, error) {
	return s.checkSpan(uint32(len(lits)), satSolverLiteralSpan{literals: lits})
}
func (s *Solver) checkSpan(count uint32, span satSolverLiteralSpan) (lbool.LBool, error) {
	if e := s.open(); e != nil {
		return lbool.Undef, e
	}
	if e := s.popToBase(); e != nil {
		return lbool.Undef, e
	}
	if e := s.emitCheckNotice(); e != nil {
		return lbool.Undef, e
	}
	if satStorageSourceDebug && s.Variables.scopeLevel != 0 {
		return lbool.Undef, ErrSATAssertion
	}
	cfg, e := s.config.State()
	if e != nil {
		return lbool.Undef, e
	}
	if cfg.DimacsDisplay {
		if s.dimacsOutput == nil {
			return lbool.Undef, ErrSATMissingDimacsBinding
		}
		if e = s.displayDIMACS(s.dimacsOutput); e != nil {
			return lbool.Undef, e
		}
		for i := uint32(0); i < count; i++ {
			l, err := span.literal(i)
			if err != nil {
				return lbool.Undef, err
			}
			if err = satDIMACSLiteralInto(s.dimacsOutput, l); err != nil {
				return lbool.Undef, err
			}
			if err = s.dimacsOutput.AppendCString(" 0\n"); err != nil {
				return lbool.Undef, err
			}
		}
		return lbool.Undef, nil
	}
	if cfg.NumParallel > 1 && !s.conflict.parallelPresent {
		return lbool.Undef, ErrSATMissingParallelBinding
	}
	r, e := s.checkCoreSpan(count, span)
	var abort satSolverAbort
	if errors.As(e, &abort) {
		return lbool.Undef, nil
	}
	return r, e
}
func (s *Solver) checkCore(lits []Literal) (lbool.LBool, error) {
	return s.checkCoreSpan(uint32(len(lits)), satSolverLiteralSpan{literals: lits})
}
func (s *Solver) checkCoreSpan(count uint32, span satSolverLiteralSpan) (lbool.LBool, error) {
	if s.propagation.inconsistent {
		return lbool.False, nil
	}
	if e := s.gc.initSearch(); e != nil {
		return lbool.Undef, e
	}
	s.traceSearchInitialized()
	if _, e := s.Lifecycle.propagate(false); e != nil {
		return lbool.Undef, e
	}
	if s.propagation.inconsistent {
		return lbool.False, nil
	}
	if e := s.initAssumptionsSpan(count, span); e != nil {
		return lbool.Undef, e
	}
	if _, e := s.Lifecycle.propagate(false); e != nil {
		return lbool.Undef, e
	}
	yes, e := s.checkInconsistent()
	if e != nil {
		return lbool.Undef, e
	}
	if yes {
		return lbool.False, nil
	}
	if e = s.cleaner.cleanup(); e != nil {
		return lbool.Undef, e
	}
	cfg, e := s.config.State()
	if e != nil {
		return lbool.Undef, e
	}
	if cfg.MaxConflicts > 0 && cfg.BurstSearch > 0 {
		s.gc.restartThreshold = cfg.BurstSearch
		r, e := s.boundedSearch()
		if e != nil {
			return lbool.Undef, e
		}
		if r != lbool.Undef {
			return r, nil
		}
		if e = s.conflict.popReinit(s.Variables.scopeLevel); e != nil {
			return lbool.Undef, e
		}
		s.conflict.conflictsSinceRestart = 0
		s.gc.restartThreshold = cfg.RestartInitial
	}
	if e = s.simplifyProblem(); e != nil {
		return lbool.Undef, e
	}
	yes, e = s.checkInconsistent()
	if e != nil {
		return lbool.Undef, e
	}
	if yes {
		return lbool.False, nil
	}
	if cfg.MaxConflicts == 0 {
		if e = s.emitSearchAbort("max-conflicts = 0", false); e != nil {
			return lbool.Undef, e
		}
		return lbool.Undef, nil
	}
	for {
		if satStorageSourceDebug && s.propagation.inconsistent {
			return lbool.Undef, ErrSATAssertion
		}
		r, e := s.boundedSearch()
		if e != nil {
			return lbool.Undef, e
		}
		if r != lbool.Undef {
			return r, nil
		}
		if s.conflict.conflicts > cfg.MaxConflicts {
			if e = s.emitSearchAbort("max-conflicts = ", true); e != nil {
				return lbool.Undef, e
			}
			return lbool.Undef, nil
		}
		if e = s.gc.restart(); e != nil {
			return lbool.Undef, e
		}
		if e = s.simplifyProblem(); e != nil {
			return lbool.Undef, e
		}
		yes, e = s.checkInconsistent()
		if e != nil {
			return lbool.Undef, e
		}
		if yes {
			return lbool.False, nil
		}
		if e = s.gc.gc(); e != nil {
			return lbool.Undef, e
		}
		if cfg.RestartMax <= s.gc.restarts {
			if e = s.emitSearchAbort("max-restarts", false); e != nil {
				return lbool.Undef, e
			}
			return lbool.Undef, nil
		}
	}
}
