package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:24-65,1806-1843;
// concrete config_mpfx breadth-first search loop.

import "github.com/glycerine/g3/pkg/arena"

func (c *FixedContext) removeFixedLeaf(n *FixedNode) {
	for i, leaf := range c.leafQueue {
		if leaf == n {
			copy(c.leafQueue[i:], c.leafQueue[i+1:])
			last := len(c.leafQueue) - 1
			c.leafQueue[last] = nil
			c.leafQueue = c.leafQueue[:last]
			return
		}
	}
}

func fixedNodeDepth(n *FixedNode) uint32 {
	var depth uint32
	for n.parent != nil {
		depth++
		n = n.parent
	}
	return depth
}

// Search initializes the root and processes the oldest leaf first. A stopped
// propagation leaves its source queue prefix pending and drops the local leaf.
// Children created by this loop remain context-owned until Close.
func (c *FixedContext) Search() error {
	if err := c.InitializeRoot(); err != nil {
		return err
	}
	if err := c.traceSearchStatistics(1809); err != nil {
		return err
	}
	if err := c.traceSearchParams(); err != nil {
		return err
	}
	if !c.searchStarted {
		if c.root.children != 0 {
			return arena.ErrInvalidSlot
		}
		c.leafQueue = []*FixedNode{c.root}
		c.searchStarted = true
	}
	for c.searchNode != nil || len(c.leafQueue) != 0 {
		if c.searchNode == nil {
			if err := sourceCheckpoint(c.limit, nil, nil, c.maxMemory, ErrFixedContextCanceled, c.cooperation); err != nil {
				return err
			}
			if len(c.queue) != 0 || c.queueHead != 0 {
				return arena.ErrInvalidSlot
			}
			if c.numNodes > c.maxNodes {
				break
			}
			n := c.leafQueue[0]
			if err := c.traceSearchNode(n, "selected", 1819); err != nil {
				return err
			}
			c.searchNode = n
			c.leafQueue[0] = nil
			c.leafQueue = c.leafQueue[1:]
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
		if n.inconsistent {
			if err := c.traceSearchNode(n, "inconsistent", 1828); err != nil {
				return err
			}
			c.searchNode = nil
			continue
		}
		if fixedNodeDepth(n) >= c.maxDepth {
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
			left, right, err := c.SplitMidpoint(n, variable, true, 1)
			for _, child := range []*FixedNode{left, right} {
				if child != nil {
					c.searchOwned = append(c.searchOwned, child)
					if !child.inconsistent {
						c.leafQueue = append(c.leafQueue, child)
					}
				}
			}
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
