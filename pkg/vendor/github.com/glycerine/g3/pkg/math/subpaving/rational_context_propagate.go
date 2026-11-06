package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:575-587,1741-1752,
// concrete config_mpq bound queue and insertion-order watched dispatch.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

// PropagateQueued processes the exact bound queue prefix. A canceled
// checkpoint leaves the next bound queued; a conflict clears the suffix.
func (c *RationalContext) PropagateQueued(n *RationalNode) error {
	if err := c.open(); err != nil {
		return err
	}
	if n == nil || n.context != c || !n.registered {
		return arena.ErrInvalidSlot
	}
	if err := n.open(); err != nil {
		return err
	}
	for c.queueHead < len(c.queue) && n.conflictVar == math.MaxUint32 {
		if err := sourceCheckpoint(c.limit, c.runtime, c.process, c.maxMemory, ErrRationalContextCanceled, c.cooperation); err != nil {
			return err
		}
		b := c.queue[c.queueHead]
		found := false
		for trail := n.trail; trail != nil; trail = trail.previous {
			if trail == b {
				found = true
				break
			}
		}
		if !found {
			return arena.ErrInvalidSlot
		}
		c.queueHead++
		if err := c.tracePropagatingBound(b); err != nil {
			return err
		}
		for _, watch := range c.watchOrder[b.variable] {
			if n.conflictVar != math.MaxUint32 {
				break
			}
			if watch.clause != nil {
				if err := c.traceMayPropagateClause(b, watch.clause); err != nil {
					return err
				}
				if b.timestamp >= watch.clause.visited {
					if err := c.PropagateClause(watch.clause, n); err != nil {
						return err
					}
				}
			} else if p := c.polynomials[watch.definition]; p != nil {
				if err := c.traceMayPropagateDefinition(b, watch.definition, p.visited); err != nil {
					return err
				}
				if b.timestamp >= p.visited {
					if err := c.PropagatePolynomial(watch.definition, n); err != nil {
						return err
					}
				}
			} else if m := c.monomials[watch.definition]; m != nil {
				if err := c.traceMayPropagateDefinition(b, watch.definition, m.visited); err != nil {
					return err
				}
				if b.timestamp >= m.visited {
					if err := c.PropagateMonomial(watch.definition, n); err != nil {
						return err
					}
				}
			} else {
				return arena.ErrInvalidSlot
			}
		}
		if n.conflictVar == math.MaxUint32 {
			if p := c.polynomials[b.variable]; p != nil {
				if err := c.traceMayPropagateDefinition(b, b.variable, p.visited); err != nil {
					return err
				}
				if b.timestamp >= p.visited {
					if err := c.PropagatePolynomial(b.variable, n); err != nil {
						return err
					}
				}
			}
			if m := c.monomials[b.variable]; m != nil {
				if err := c.traceMayPropagateDefinition(b, b.variable, m.visited); err != nil {
					return err
				}
				if b.timestamp >= m.visited {
					if err := c.PropagateMonomial(b.variable, n); err != nil {
						return err
					}
				}
			}
		}
	}
	c.queue, c.queueHead = nil, 0
	return nil
}
