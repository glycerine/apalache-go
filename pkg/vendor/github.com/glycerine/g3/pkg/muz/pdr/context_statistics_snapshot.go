package pdr

// Source: context::collect_statistics/reset_statistics and manager lifetime
// across context::reset in src/muz/pdr/pdr_context.cpp.

import "github.com/glycerine/g3/pkg/util/statistics"

// StatisticsSnapshot carries counters whose source owners survive a program
// reset. Transformer properties and cache counters belong to the live program.
type StatisticsSnapshot struct {
	Solvers    statistics.Statistics
	Unfoldings uint32
	MaxDepth   uint32
}

func (s *StatisticsSnapshot) Merge(other StatisticsSnapshot) {
	s.Solvers.Copy(other.Solvers)
	s.Unfoldings += other.Unfoldings
	if other.MaxDepth > s.MaxDepth {
		s.MaxDepth = other.MaxDepth
	}
}

// Reset follows context::reset_statistics: kernel reset_statistics is a no-op.
func (s *StatisticsSnapshot) Reset() { s.Unfoldings, s.MaxDepth = 0, 0 }

func (s *StatisticsSnapshot) Collect(destination *statistics.Statistics) error {
	if s == nil || destination == nil {
		return ErrContextOwner
	}
	destination.Copy(s.Solvers)
	destination.UpdateUint("PDR num unfoldings", s.Unfoldings)
	destination.UpdateUint("PDR max depth", s.MaxDepth)
	return nil
}

func (c *Context) SnapshotStatistics() (StatisticsSnapshot, error) {
	if err := c.open(); err != nil {
		return StatisticsSnapshot{}, err
	}
	result := StatisticsSnapshot{Unfoldings: c.nodes, MaxDepth: c.maxDepth}
	err := c.owner.CollectStatistics(&result.Solvers)
	return result, err
}
