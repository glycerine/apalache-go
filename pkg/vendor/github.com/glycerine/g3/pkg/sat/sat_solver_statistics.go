package sat

// Source: src/sat/sat_solver.cpp:2781-2801,3716-3751; src/sat/sat_solver.h:48-70.

import (
	"github.com/glycerine/g3/pkg/util/statistics"
)

// Source-order keys for the solver's statistics counters.

type satSolverStatisticsKeys struct {
	main                [15]statistics.Key
	cleaner             [2]statistics.Key
	simplifier          [5]statistics.Key
	scc, asymm, probing statistics.Key
}

func (s *Solver) collectMainStatistics(st *statistics.Statistics, keys [15]statistics.Key) error {
	values := [15]uint32{
		s.Variables.mkVarCount, s.Lifecycle.mkBinary, s.Lifecycle.mkTernary,
		s.Lifecycle.mkNary, s.gc.gcClauseStat, s.Lifecycle.deleted,
		s.conflict.conflictStat, s.propagation.clausePropagations,
		s.decision.decisions, s.propagation.binaryPropagations,
		s.propagation.ternaryPropagations, s.gc.restartStat,
		s.analysis.minimizedLits, s.dynamic.eliminated, s.blockedCorrectionSets,
	}
	for i, value := range values {
		st.UpdateUint(keys[i], value)
	}
	return nil
}
func (s *Solver) resetMainStatistics() {
	s.Variables.mkVarCount = 0
	s.Lifecycle.mkBinary = 0
	s.Lifecycle.mkTernary = 0
	s.Lifecycle.mkNary = 0
	s.conflict.conflictStat = 0
	s.propagation.clausePropagations = 0
	s.propagation.binaryPropagations = 0
	s.propagation.ternaryPropagations = 0
	s.decision.decisions = 0
	s.gc.restartStat = 0
	s.gc.gcClauseStat = 0
	s.Lifecycle.deleted = 0
	s.analysis.minimizedLits = 0
	s.dynamic.eliminated = 0
	s.Lifecycle.nonLearnedGeneration = 0
	s.blockedCorrectionSets = 0
}
func (s *Solver) collectStatistics(st *statistics.Statistics, keys satSolverStatisticsKeys) error {
	if e := s.open(); e != nil {
		return e
	}
	if e := s.collectMainStatistics(st, keys.main); e != nil {
		return e
	}
	if e := s.cleaner.collectStatistics(st, keys.cleaner[0], keys.cleaner[1]); e != nil {
		return e
	}
	if e := s.simplifier.collectStatistics(st, keys.simplifier); e != nil {
		return e
	}
	if e := s.scc.collectStatistics(st, keys.scc); e != nil {
		return e
	}
	if e := s.asymm.collectStatistics(st, keys.asymm); e != nil {
		return e
	}
	return s.probing.collectStatistics(st, keys.probing)
}
func (s *Solver) resetStatistics() error {
	if e := s.open(); e != nil {
		return e
	}
	s.resetMainStatistics()
	s.cleaner.resetStatistics()
	s.simplifier.resetStatistics()
	// Source deliberately has no m_scc.reset_statistics() at this site.
	s.asymm.resetStatistics()
	s.probing.resetStatistics()
	return nil
}
