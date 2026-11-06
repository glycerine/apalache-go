package spacer

// Source: spacer_context.cpp context::reset/collect_statistics/reset_statistics
// and spacer_virtual_solver.cpp virtual_solver_factory::reset_statistics.

import "github.com/glycerine/g3/pkg/util/statistics"

// StatisticsSnapshot owns contributions from owners that survive a source
// program reset. Predicate frame/propagation gauges remain with the live program.
type StatisticsSnapshot struct {
	Solvers                 statistics.Statistics
	Virtual                 statistics.Statistics
	Timers                  statistics.Statistics
	counters                contextStatistics
	cexDepth                uint32
	cexDepthSet             bool
	generalizers            boolGeneralizerStatistics
	generalizersInitialized bool
}

func (s *StatisticsSnapshot) Merge(other StatisticsSnapshot) {
	s.Solvers.Copy(other.Solvers)
	s.Virtual.Copy(other.Virtual)
	s.Timers.Copy(other.Timers)
	// update_rules recreates the generalizer owner before attempting rule
	// installation. Earlier preparation failures leave the preceding owner live.
	if other.generalizersInitialized {
		s.generalizers = other.generalizers
		s.generalizersInitialized = true
	}
	s.counters.queries += other.counters.queries
	s.counters.reachQueries += other.counters.reachQueries
	s.counters.reuseReach += other.counters.reuseReach
	s.counters.expandUndef += other.counters.expandUndef
	s.counters.lemmas += other.counters.lemmas
	s.counters.restarts += other.counters.restarts
	if other.counters.maxDepth > s.counters.maxDepth {
		s.counters.maxDepth = other.counters.maxDepth
	}
	// Source assigns the latest attempted level despite this statistic's name.
	if other.counters.queryLevelSet {
		s.counters.maxQueryLevel = other.counters.maxQueryLevel
		s.counters.queryLevelSet = true
	}
	if other.cexDepthSet {
		s.cexDepth = other.cexDepth
		s.cexDepthSet = true
	}
}

// Reset clears factory/search counters; the native SMT kernel reset is a no-op.
func (s *StatisticsSnapshot) Reset() {
	s.Virtual.Reset()
	s.Timers.Reset()
	s.generalizers = boolGeneralizerStatistics{}
	s.counters = contextStatistics{}
	s.cexDepth, s.cexDepthSet = 0, false
}

func (s *StatisticsSnapshot) Collect(destination *statistics.Statistics) error {
	if s == nil || destination == nil {
		return ErrContextOwner
	}
	destination.Copy(s.Solvers)
	destination.Copy(s.Virtual)
	destination.Copy(s.Timers)
	s.generalizers.collect(destination)
	destination.UpdateUint("SPACER num queries", s.counters.queries)
	destination.UpdateUint("SPACER num reach queries", s.counters.reachQueries)
	destination.UpdateUint("SPACER num reuse reach facts", s.counters.reuseReach)
	destination.UpdateUint("SPACER max query lvl", s.counters.maxQueryLevel)
	destination.UpdateUint("SPACER max depth", s.counters.maxDepth)
	destination.UpdateUint("SPACER cex depth", s.cexDepth)
	destination.UpdateUint("SPACER expand node undef", s.counters.expandUndef)
	destination.UpdateUint("SPACER num lemmas", s.counters.lemmas)
	destination.UpdateUint("SPACER restarts", s.counters.restarts)
	return nil
}

func (c *Context) SnapshotStatistics() (StatisticsSnapshot, error) {
	if err := c.open(); err != nil {
		return StatisticsSnapshot{}, err
	}
	result := StatisticsSnapshot{counters: c.stats, cexDepth: c.cexDepth, cexDepthSet: c.cexDepthSet, generalizers: c.generalizerStats, generalizersInitialized: c.generalizersInitialized}
	if err := c.timers.collect(&result.Timers); err != nil {
		return result, err
	}
	for _, pool := range c.owner.contexts {
		for _, factory := range pool.factories {
			if err := factory.backend.CollectStatistics(&result.Solvers); err != nil {
				return result, err
			}
			if err := factory.collectVirtualStatistics(&result.Virtual); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}
