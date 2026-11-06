package opt

// Source: src/opt/maxsmt.h collect_statistics and engine-specific statistics.

type MaxSMTStatistics struct {
	Engine                 string
	Checks                 uint32
	Inputs                 uint32
	Comparators            uint32
	Candidates             uint32
	Improvements           uint32
	Cores                  uint32
	CoreMinimizationChecks uint32
	Assumptions            uint32
	Resolutions            uint32
	CorrectionSets         uint32
	CorrectionResolutions  uint32
	Pivots                 uint32
	UpperBoundBlocks       uint32
	Mutexes                uint32
	MutexTerms             uint32
	Auxiliaries            uint32
	NetworkClauses         uint32
	SLSAttempts            uint32
	SLSChecks              uint32
	SLSFlips               uint32
	SLSImprovements        uint32
}
