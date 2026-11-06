package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:310-342,1661-1681,
// concrete config_mpf child views and recent decision-bound replay.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// NewChildNode copies the parent's bound-pointer views and trail. Later
// publications replace only the child's view; the parent retains its bounds.
func (c *ArbitraryContext) NewChildNode(parent *ArbitraryNode) (*ArbitraryNode, error) {
	if err := c.validNode(parent); err != nil {
		return nil, err
	}
	if parent.conflictVar != math.MaxUint32 || parent.depth == math.MaxUint32 || parent.children == math.MaxUint32 || len(c.nodes) >= math.MaxUint32 {
		return nil, arena.ErrInvalidSlot
	}
	n := &ArbitraryNode{
		context: c, parent: parent, id: uint32(len(c.nodes)), depth: parent.depth + 1,
		lower: append([]*ArbitraryBound(nil), parent.lower...),
		upper: append([]*ArbitraryBound(nil), parent.upper...),
		trail: parent.trail, conflictVar: parent.conflictVar,
	}
	parent.children++
	c.nodes = append(c.nodes, n)
	c.pushLeafFront(n)
	return n, nil
}

// DecideBound creates the splitter's unqueued bound. Replay later selects the
// surviving child-local bounds and assigns the current context timestamp.
func (c *ArbitraryContext) DecideBound(n *ArbitraryNode, variable uint32, value *numeral.ArbitraryFloat, lower, open bool) error {
	if err := c.checkNode(n, variable); err != nil {
		return err
	}
	if n.parent == nil || len(c.queue) != 0 || c.queueHead != 0 {
		return arena.ErrInvalidSlot
	}
	return c.publishBoundWithReason(n, variable, value, lower, open, true, nil, 0, false, false)
}

func (c *ArbitraryContext) boundOnTrail(n *ArbitraryNode, b *ArbitraryBound) bool {
	for current := n.trail; current != nil; current = current.previous {
		if current == b {
			return true
		}
	}
	return false
}

// AddRecentBounds replays surviving child-local bounds newest first, without
// advancing the context timestamp or replaying a superseded bound.
func (c *ArbitraryContext) AddRecentBounds(n *ArbitraryNode) error {
	if err := c.validNode(n); err != nil {
		return err
	}
	if n.parent == nil || len(c.queue) != 0 || c.queueHead != 0 {
		return arena.ErrInvalidSlot
	}
	boundary := n.parent.trail
	var recent []*ArbitraryBound
	current := n.trail
	for current != boundary && current != nil {
		if current.variable >= uint32(len(n.lower)) {
			return arena.ErrInvalidSlot
		}
		if current.lower && n.lower[current.variable] == current || !current.lower && n.upper[current.variable] == current {
			recent = append(recent, current)
		}
		current = current.previous
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
