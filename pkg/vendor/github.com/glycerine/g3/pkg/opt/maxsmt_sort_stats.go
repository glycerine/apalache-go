package opt

// Source: src/opt/sortmax.cpp retained progress counters.

type sortMaxStatistics struct {
	Inputs, Comparators, Checks, Improvements uint32
	Auxiliaries, Clauses                      uint32
	Mutexes, MutexTerms                       uint32
}

func (e *sortMaxEngine) statistics() MaxSMTStatistics {
	return MaxSMTStatistics{Engine: "sortmax", Checks: e.stats.Checks, Inputs: e.stats.Inputs, Comparators: e.stats.Comparators, Improvements: e.stats.Improvements, Auxiliaries: e.stats.Auxiliaries, NetworkClauses: e.stats.Clauses, Mutexes: e.stats.Mutexes, MutexTerms: e.stats.MutexTerms}
}
