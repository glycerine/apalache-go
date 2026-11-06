package opt

// Source: src/opt/maxres.cpp maxres::stats.

type maxResStatistics struct {
	Cores, Checks, Assumptions, Resolutions           uint32
	Candidates, Improvements, CoreMinChecks           uint32
	CorrectionSets, CorrectionResolutions, Pivots     uint32
	UpperBoundBlocks                                   uint32
	Mutexes, MutexTerms                               uint32
	Auxiliaries                                       uint32
	SLSAttempts, SLSChecks, SLSFlips, SLSImprovements uint32
}

func (e *maxResEngine) statistics() MaxSMTStatistics {
	return MaxSMTStatistics{
		Engine: e.strategy.name(), Checks: e.stats.Checks, Candidates: e.stats.Candidates,
		Improvements: e.stats.Improvements, Cores: e.stats.Cores,
		CoreMinimizationChecks: e.stats.CoreMinChecks, Assumptions: e.stats.Assumptions,
		Resolutions: e.stats.Resolutions, CorrectionSets: e.stats.CorrectionSets,
		CorrectionResolutions: e.stats.CorrectionResolutions, Pivots: e.stats.Pivots,
		UpperBoundBlocks: e.stats.UpperBoundBlocks,
		Mutexes: e.stats.Mutexes, MutexTerms: e.stats.MutexTerms, Auxiliaries: e.stats.Auxiliaries,
		SLSAttempts: e.stats.SLSAttempts, SLSChecks: e.stats.SLSChecks,
		SLSFlips: e.stats.SLSFlips, SLSImprovements: e.stats.SLSImprovements,
	}
}
