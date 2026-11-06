package duality

import (
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/statistics"
)

// nativeSolverDecisions reads the same cumulative statistic as the source
// wrapper's display/atoi path, directly from Go's typed statistics container.
// Source: duality_wrapper.cpp:311-324. Missing decisions means zero.
func nativeSolverDecisions(native solver.Solver) (int, error) {
	if native == nil {
		return 0, ErrWrapperOwner
	}
	stats := statistics.New()
	if err := native.CollectStatistics(stats); err != nil {
		return 0, err
	}
	var total uint32
	for i := 0; i < stats.Size(); i++ {
		if stats.Key(i) == "decisions" && stats.IsUint(i) {
			total += stats.UintValue(i)
		}
	}
	return int(total), nil
}
