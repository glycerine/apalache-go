package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:24-101,344-359,
// concrete config_hwf breadth-first node and round-robin variable selectors.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

func (c *HardwareContext) pushLeafFront(n *HardwareNode) {
	n.leafPrev = nil
	n.leafNext = c.leafHead
	if c.leafHead != nil {
		c.leafHead.leafPrev = n
	} else {
		c.leafTail = n
	}
	c.leafHead = n
	n.queuedLeaf = true
}

func (c *HardwareContext) removeLeaf(n *HardwareNode) {
	if !n.queuedLeaf {
		return
	}
	if n.leafPrev != nil {
		n.leafPrev.leafNext = n.leafNext
	} else {
		c.leafHead = n.leafNext
	}
	if n.leafNext != nil {
		n.leafNext.leafPrev = n.leafPrev
	} else {
		c.leafTail = n.leafPrev
	}
	n.leafPrev, n.leafNext = nil, nil
	n.queuedLeaf = false
}

// SelectLeafBFS returns the oldest queued leaf, as the source's default
// breadth-first selector does. An empty leaf list returns nil.
func (c *HardwareContext) SelectLeafBFS() (*HardwareNode, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.leafTail, nil
}

// PopLeafBFS selects and removes the oldest leaf before its propagation and
// split, matching the retained search loop's leaf-list mutation order.
func (c *HardwareContext) PopLeafBFS() (*HardwareNode, error) {
	n, err := c.SelectLeafBFS()
	if err != nil || n == nil {
		return n, err
	}
	c.removeLeaf(n)
	return n, nil
}

// SelectBranchVariable scans from the variable after the most recent axiom
// bound in a child. The root starts at zero; definitions are skipped by the
// source default selector.
func (c *HardwareContext) SelectBranchVariable(n *HardwareNode, onlyNonDefinition bool) (variable uint32, found bool, err error) {
	if err = c.open(); err != nil {
		return 0, false, err
	}
	if n == nil || n.context != c || len(c.isInt) == 0 || n.conflictVar != math.MaxUint32 || len(c.queue) != 0 || c.queueHead != 0 {
		return 0, false, arena.ErrInvalidSlot
	}
	if err = n.open(); err != nil {
		return 0, false, err
	}
	start := uint32(0)
	if n != c.root {
		decision := n.trail
		for decision != nil && decision.reason != hardwareReasonAxiom {
			decision = decision.prev
		}
		if decision == nil {
			return 0, false, arena.ErrInvalidSlot
		}
		start = decision.variable + 1
		if start == uint32(len(c.isInt)) {
			start = 0
		}
	}
	x := start
	for {
		if !onlyNonDefinition || c.polynomials[x] == nil && c.monomials[x] == nil {
			lower, upper := n.lower[x], n.upper[x]
			if lower == nil || upper == nil || !c.numbers.Equal(&lower.value, &upper.value) {
				return x, true, nil
			}
		}
		x++
		if x == uint32(len(c.isInt)) {
			x = 0
		}
		if x == start {
			return 0, false, nil
		}
	}
}
