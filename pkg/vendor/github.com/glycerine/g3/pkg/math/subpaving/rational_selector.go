package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:967-1060, concrete config_mpq.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

func (c *RationalContext) pushLeafFront(n *RationalNode) {
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

func (c *RationalContext) removeLeaf(n *RationalNode) {
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

func (c *RationalContext) SelectLeafBFS() (*RationalNode, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.leafTail, nil
}

func (c *RationalContext) PopLeafBFS() (*RationalNode, error) {
	n, err := c.SelectLeafBFS()
	if err != nil || n == nil {
		return n, err
	}
	c.removeLeaf(n)
	return n, nil
}

// SelectBranchVariable follows the source round-robin selector. A child starts
// immediately after its newest axiom decision; the root starts at variable 0.
func (c *RationalContext) SelectBranchVariable(n *RationalNode, onlyNonDefinition bool) (uint32, bool, error) {
	if err := c.open(); err != nil {
		return 0, false, err
	}
	if n == nil || n.context != c || !n.registered || len(c.variables) == 0 || n.conflictVar != math.MaxUint32 || len(c.queue) != 0 || c.queueHead != 0 {
		return 0, false, arena.ErrInvalidSlot
	}
	if err := n.open(); err != nil {
		return 0, false, err
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
			lo, hi := n.lower[x], n.upper[x]
			if lo == nil || hi == nil {
				return x, true, nil
			}
			equal, err := c.numbers.Equal(&lo.value, &hi.value)
			if err != nil {
				return 0, false, err
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
