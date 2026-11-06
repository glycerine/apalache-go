package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:527,888-891,1163-1167,
// 1869-1884; concrete config_mpfx statistics.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/statistics"
)

// ResetStatistics clears event counts while retaining the live node count.
func (c *FixedContext) ResetStatistics() error {
	if err := c.open(); err != nil {
		return err
	}
	c.numConflicts = 0
	c.numBounds = 0
	c.numSplits = 0
	c.numVisited = 0
	return nil
}

// CollectStatistics appends the five source counters in source order.
func (c *FixedContext) CollectStatistics(st *statistics.Statistics) error {
	if err := c.open(); err != nil {
		return err
	}
	if st == nil {
		return arena.ErrInvalidSlot
	}
	st.UpdateUint64("conflicts", c.numConflicts)
	st.UpdateUint64("new bounds", c.numBounds)
	st.UpdateUint64("splits", c.numSplits)
	st.UpdateUint("nodes", c.numNodes)
	st.UpdateUint64("visited", c.numVisited)
	return nil
}
