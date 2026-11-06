package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:66-101; concrete config_mpff
// round-robin variable selector.

import "github.com/glycerine/g3/pkg/arena"

// SelectBranchVariable starts after the latest child axiom decision, wrapping
// once through the current variables. The default selector skips definitions.
func (c *FastContext) SelectBranchVariable(n *FastNode, onlyNonDefinition bool) (variable uint32, found bool, err error) {
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
		for decision != nil && decision.reason.kind != fastReasonAxiom {
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
			equal, e := c.numbers.Equal(lower.value, upper.value)
			if e != nil {
				return 0, false, e
			}
			if !equal {
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
