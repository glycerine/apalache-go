package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:1170-1175,1683-1802,
// concrete config_hwf polynomial watchers and bound queue.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (c *HardwareContext) definitionVisited(variable uint32) (uint64, bool) {
	if m := c.monomials[variable]; m != nil {
		return m.visited, true
	}
	if p := c.polynomials[variable]; p != nil {
		return p.visited, true
	}
	return 0, false
}

func (c *HardwareContext) propagateDefinition(variable uint32, n *HardwareNode) error {
	c.numVisited++
	if c.monomials[variable] != nil {
		return c.PropagateMonomial(variable, n)
	}
	if c.polynomials[variable] != nil {
		return c.PropagatePolynomial(variable, n)
	}
	return arena.ErrInvalidSlot
}

// PropagateAllDefinitions visits definitions in variable-creation order.
func (c *HardwareContext) PropagateAllDefinitions(n *HardwareNode) error {
	if err := c.open(); err != nil {
		return err
	}
	if n == nil || n.context != c {
		return arena.ErrInvalidSlot
	}
	if err := n.open(); err != nil {
		return err
	}
	for x := range c.isInt {
		if n.conflictVar != math.MaxUint32 {
			break
		}
		variable := uint32(x)
		if _, ok := c.definitionVisited(variable); ok {
			if err := c.propagateDefinition(variable, n); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *HardwareContext) boundOnTrail(n *HardwareNode, b *HardwareBound) bool {
	for current := n.trail; current != nil; current = current.prev {
		if current == b {
			return true
		}
	}
	return false
}

// AddRecentBounds replays only surviving child-local bounds, newest first.
// The source assigns the current context timestamp without incrementing it.
func (c *HardwareContext) AddRecentBounds(n *HardwareNode) error {
	if err := c.open(); err != nil {
		return err
	}
	if n == nil || n.context != c || n.parent == nil || len(c.queue) != 0 || c.queueHead != 0 {
		return arena.ErrInvalidSlot
	}
	if err := n.open(); err != nil {
		return err
	}
	boundary := n.parent.trail
	current := n.trail
	var recent []*HardwareBound
	for current != boundary && current != nil {
		b := current
		if b.variable >= uint32(len(n.lower)) {
			return arena.ErrInvalidSlot
		}
		if b.lower && n.lower[b.variable] == b || !b.lower && n.upper[b.variable] == b {
			recent = append(recent, b)
		}
		current = current.prev
	}
	if current != boundary {
		return arena.ErrInvalidSlot
	}
	for _, b := range recent {
		b.timestamp = c.timestamp
		c.queue = append(c.queue, b)
	}
	return nil
}

func (c *HardwareContext) processQueuedBound(n *HardwareNode, b *HardwareBound) error {
	if b == nil || b.variable >= uint32(len(c.watchers)) || !c.boundOnTrail(n, b) {
		return arena.ErrInvalidSlot
	}
	if err := c.tracePropagatingBound(b); err != nil {
		return err
	}
	visit := func(variable uint32, catchArithmetic bool) error {
		visited, ok := c.definitionVisited(variable)
		if !ok {
			return nil
		}
		if err := c.traceMayPropagateDefinition(b, variable, visited); err != nil {
			if catchArithmetic && errors.Is(err, numeral.ErrHardwareFloatNonRegular) {
				c.arithmeticFailed = true
				return nil
			}
			return err
		}
		if b.timestamp < visited {
			return nil
		}
		if err := c.propagateDefinition(variable, n); err != nil {
			if catchArithmetic && errors.Is(err, numeral.ErrHardwareFloatNonRegular) {
				c.arithmeticFailed = true
				return nil
			}
			return err
		}
		return nil
	}
	for _, watched := range c.watchOrder[b.variable] {
		if n.conflictVar != math.MaxUint32 {
			return nil
		}
		if watched.clause != nil {
			clause := watched.clause
			if err := c.traceMayPropagateClause(b, clause); err != nil {
				if errors.Is(err, numeral.ErrHardwareFloatNonRegular) {
					c.arithmeticFailed = true
					continue
				}
				return err
			}
			if b.timestamp < clause.visited {
				continue
			}
			if err := c.PropagateClause(clause, n); err != nil {
				if errors.Is(err, numeral.ErrHardwareFloatNonRegular) {
					c.arithmeticFailed = true
					continue
				}
				return err
			}
		} else if err := visit(watched.definition, true); err != nil {
			return err
		}
	}
	if n.conflictVar != math.MaxUint32 {
		return nil
	}
	if _, ok := c.definitionVisited(b.variable); ok {
		return visit(b.variable, false)
	}
	return nil
}

// PropagateQueued drains the live queue, including bounds added by watcher
// visits. Checkpoint cancellation leaves the unprocessed suffix for resumption.
func (c *HardwareContext) PropagateQueued(n *HardwareNode) error {
	if err := c.open(); err != nil {
		return err
	}
	if n == nil || n.context != c {
		return arena.ErrInvalidSlot
	}
	if err := n.open(); err != nil {
		return err
	}
	for c.queueHead < len(c.queue) && n.conflictVar == math.MaxUint32 {
		if err := sourceCheckpoint(c.limit, nil, c.process, c.maxMemory, ErrHardwareContextCanceled, c.cooperation); err != nil {
			return err
		}
		b := c.queue[c.queueHead]
		c.queueHead++
		if err := c.processQueuedBound(n, b); err != nil {
			return err
		}
	}
	c.queue = nil
	c.queueHead = 0
	return nil
}
