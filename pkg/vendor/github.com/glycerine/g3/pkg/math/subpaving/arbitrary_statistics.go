package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1164-1174,1869-1884,
// concrete config_mpf bound, conflict, split, node and visit counters.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/statistics"
)

// ResetStatistics clears event counters while preserving the live node count.
func (c *ArbitraryContext) ResetStatistics() error {
	if err := c.open(); err != nil {
		return err
	}
	c.numConflicts = 0
	c.numBounds = 0
	c.numSplits = 0
	c.numVisited = 0
	return nil
}

// CollectStatistics appends the source's five counters in source order.
func (c *ArbitraryContext) CollectStatistics(st *statistics.Statistics) error {
	if err := c.open(); err != nil {
		return err
	}
	if st == nil {
		return arena.ErrInvalidSlot
	}
	st.UpdateUint64("conflicts", c.numConflicts)
	st.UpdateUint64("new bounds", c.numBounds)
	st.UpdateUint64("splits", c.numSplits)
	st.UpdateUint("nodes", uint32(len(c.nodes)))
	st.UpdateUint64("visited", c.numVisited)
	return nil
}
