package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1806-1843, concrete
// config_hwf default breadth-first search loop.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

// Search initializes the root, then processes breadth-first leaves using the
// default midpoint splitter and round-robin variable selector. If propagation
// fails after a child is removed from the leaf list, the source leaves its
// queue prefix in place and does not retain that local selected node.
func (c *HardwareContext) Search() error {
	if err := c.InitializeRoot(); err != nil {
		return err
	}
	if err := c.traceSearchStatistics(1809); err != nil {
		return err
	}
	if err := c.traceSearchParams(); err != nil {
		return err
	}
	for c.searchNode != nil || c.leafHead != nil {
		if c.searchNode == nil {
			if err := sourceCheckpoint(c.limit, nil, c.process, c.maxMemory, ErrHardwareContextCanceled, c.cooperation); err != nil {
				return err
			}
			if len(c.queue) != 0 || c.queueHead != 0 {
				return arena.ErrInvalidSlot
			}
			if c.activeNodes > c.maxNodes {
				break
			}
			n, err := c.SelectLeafBFS()
			if err != nil {
				return err
			}
			if n == nil {
				break
			}
			if err := c.traceSearchNode(n, "selected", 1819); err != nil {
				return err
			}
			c.removeLeaf(n)
			c.searchNode = n
			c.searchPhase = 0
		}
		n := c.searchNode
		if c.searchPhase == 0 {
			if n != c.root {
				if err := c.AddRecentBounds(n); err != nil {
					c.searchNode, c.searchPhase = nil, 0
					return err
				}
			}
			c.searchPhase = 1
		}
		if c.searchPhase == 1 {
			if n != c.root {
				if err := c.PropagateQueued(n); err != nil {
					c.searchNode, c.searchPhase = nil, 0
					return err
				}
			}
			c.searchPhase = 2
		}
		if c.searchPhase == 2 {
			if err := c.traceSearchNode(n, "after", 1825); err != nil {
				return err
			}
			c.searchPhase = 3
		}
		if n.conflictVar != math.MaxUint32 {
			if err := c.traceSearchNode(n, "inconsistent", 1828); err != nil {
				return err
			}
			c.searchNode = nil
			continue
		}
		if n.depth >= c.maxDepth {
			if err := c.traceSearchNode(n, "depth", 1833); err != nil {
				return err
			}
			c.searchNode = nil
			continue
		}
		variable, found := uint32(0), false
		if len(c.isInt) != 0 {
			var err error
			variable, found, err = c.SelectBranchVariable(n, true)
			if err != nil {
				return err
			}
		}
		if err := c.traceSearchVariable(variable, found); err != nil {
			return err
		}
		if found {
			_, _, err := c.SplitMidpoint(n, variable, true, 1)
			if err != nil {
				c.searchNode = nil
				return err
			}
			c.numSplits++
		}
		c.searchNode = nil
	}
	return c.traceSearchStatistics(1844)
}
