package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1665-1682,1806-1843,
// concrete config_mpq recent-bound propagation and breadth-first search.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

// AddRecentBounds queues the newest bound for each affected direction since
// the parent trail. Decision bounds keep the current timestamp until selected.
func (c *RationalContext) AddRecentBounds(n *RationalNode) error {
	if err := c.open(); err != nil {
		return err
	}
	if n == nil || n.context != c || !n.registered || n.parent == nil || len(c.queue) != 0 || c.queueHead != 0 || n.conflictVar != math.MaxUint32 {
		return arena.ErrInvalidSlot
	}
	if err := n.open(); err != nil {
		return err
	}
	stop := n.parent.trail
	for b := n.trail; b != stop; b = b.previous {
		if b == nil {
			return arena.ErrInvalidSlot
		}
		if b.lower && n.lower[b.variable] != b || !b.lower && n.upper[b.variable] != b {
			continue
		}
		b.timestamp = c.timestamp
		c.queue = append(c.queue, b)
	}
	return nil
}

// Search processes breadth-first leaves using source recent-bound replay. A
// canceled propagation leaves its queue prefix pending and drops the local
// selected leaf; a later search reaches the source queue-empty invariant.
func (c *RationalContext) Search() error {
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
			if err := sourceCheckpoint(c.limit, c.runtime, c.process, c.maxMemory, ErrRationalContextCanceled, c.cooperation); err != nil {
				return err
			}
			if len(c.queue) != 0 || c.queueHead != 0 {
				return arena.ErrInvalidSlot
			}
			if uint64(len(c.nodes)) > uint64(c.maxNodes) {
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
			c.searchNode, c.searchPhase = n, 0
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
		if len(c.variables) != 0 {
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
