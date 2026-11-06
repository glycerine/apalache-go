package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:58-101,345-359,
// concrete config_mpfx default round-robin variable selector.

import "github.com/glycerine/g3/pkg/arena"

// SelectBranchVariable returns the first non-singleton variable after the
// newest split decision, wrapping around the context's variable order.
// Definitions are skipped when onlyNonDefinition is true, as in the source's
// default selector.
func (c *FixedContext) SelectBranchVariable(n *FixedNode, onlyNonDefinition bool) (variable uint32, found bool, err error) {
	if err = c.open(); err != nil {
		return 0, false, err
	}
	if n == nil || n.context != c || len(c.isInt) == 0 || n.inconsistent || len(c.queue) != 0 || c.queueHead != 0 {
		return 0, false, arena.ErrInvalidSlot
	}
	if err = n.open(); err != nil {
		return 0, false, err
	}
	start := uint32(0)
	if n != c.root {
		decision := n.trail
		for decision != nil && !decision.decision {
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
			if lower == nil || upper == nil {
				return x, true, nil
			}
			cmp, e := c.numbers.Compare(lower.value, upper.value)
			if e != nil {
				return 0, false, e
			}
			if cmp != 0 {
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
