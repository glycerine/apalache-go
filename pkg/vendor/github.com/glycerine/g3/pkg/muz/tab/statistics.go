package tab

// Source: src/muz/tab/tab_context.cpp imp::reset_statistics and
// imp::collect_statistics.

import (
	"github.com/glycerine/g3/pkg/muz/base"
	ustatistics "github.com/glycerine/g3/pkg/util/statistics"
)

// Reset clears cumulative tabulation counters.
func (s *Statistics) Reset() {
	if s != nil {
		*s = Statistics{}
	}
}

// Collect appends source-named tabulation counters to destination.
func (s Statistics) Collect(destination *ustatistics.Statistics) error {
	if destination == nil {
		return base.ErrEngineBase
	}
	destination.UpdateUint("tab.num_unfold", s.Unfold)
	destination.UpdateUint("tab.num_unfold_fail", s.UnfoldFail)
	destination.UpdateUint("tab.num_subsumed", s.Subsumed)
	return nil
}
