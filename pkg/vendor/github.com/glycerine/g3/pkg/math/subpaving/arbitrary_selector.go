package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:24-101,344-359,
// concrete config_mpf breadth-first leaf and round-robin variable selectors.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

func (c *ArbitraryContext) pushLeafFront(n *ArbitraryNode) {
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

func (c *ArbitraryContext) removeLeaf(n *ArbitraryNode) {
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

func (c *ArbitraryContext) SelectLeafBFS() (*ArbitraryNode, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.leafTail, nil
}

func (c *ArbitraryContext) PopLeafBFS() (*ArbitraryNode, error) {
	n, err := c.SelectLeafBFS()
	if err != nil || n == nil {
		return n, err
	}
	c.removeLeaf(n)
	return n, nil
}

// SelectBranchVariable scans from the variable after the most recent axiom
// bound on a child. The source root starts at zero; its default skips definitions.
func (c *ArbitraryContext) SelectBranchVariable(n *ArbitraryNode, onlyNonDefinition bool) (variable uint32, found bool, err error) {
	if err = c.validNode(n); err != nil {
		return 0, false, err
	}
	if len(c.variables) == 0 || n.conflictVar != math.MaxUint32 || len(c.queue) != 0 || c.queueHead != 0 {
		return 0, false, arena.ErrInvalidSlot
	}
	start := uint32(0)
	if n != c.root {
		decision := n.trail
		for decision != nil && !decision.axiom {
			decision = decision.previous
		}
		if decision == nil {
			return 0, false, arena.ErrInvalidSlot
		}
		start = decision.variable + 1
		if start == uint32(len(c.variables)) {
			start = 0
		}
	}
	x := start
	for {
		if !onlyNonDefinition || c.polynomials[x] == nil && c.monomials[x] == nil {
			lower, upper := n.lower[x], n.upper[x]
			if lower == nil || upper == nil {
				return x, true, nil
			}
			equal, e := c.numbers.Equal(&lower.value, &upper.value)
			if e != nil {
				return 0, false, e
			}
			if !equal {
				return x, true, nil
			}
		}
		x++
		if x == uint32(len(c.variables)) {
			x = 0
		}
		if x == start {
			return 0, false, nil
		}
	}
}
