package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:291-342,525-581,1337-1343,
// 1773-1785, concrete config_mpf root unit-bound publication.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/messages"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var ErrArbitraryContextCanceled = z3exception.NewDefaultException(messages.Canceled)
var ErrArbitraryContextTimestamp error = &sourceFailure{message: "subpaving: arbitrary bound timestamp exhausted"}

type ArbitraryBound struct {
	context          *ArbitraryContext
	variable         uint32
	value            numeral.ArbitraryFloat
	lower            bool
	open             bool
	timestamp        uint64
	previous         *ArbitraryBound
	axiom            bool
	reasonClause     *ArbitraryClause
	reasonVariable   uint32
	definitionReason bool
}

type ArbitraryNode struct {
	context      *ArbitraryContext
	parent       *ArbitraryNode
	id           uint32
	depth        uint32
	children     uint32
	leafPrev     *ArbitraryNode
	leafNext     *ArbitraryNode
	queuedLeaf   bool
	lower, upper []*ArbitraryBound
	trail        *ArbitraryBound
	conflictVar  uint32
	closed       bool
}

func (c *ArbitraryContext) validNode(n *ArbitraryNode) error {
	if err := c.open(); err != nil {
		return err
	}
	if n == nil || n.context != c || n.closed || n.id >= uint32(len(c.nodes)) || c.nodes[n.id] != n {
		return arena.ErrInvalidSlot
	}
	return nil
}

func (c *ArbitraryContext) checkNode(n *ArbitraryNode, variable uint32) error {
	if err := c.validNode(n); err != nil {
		return err
	}
	if variable >= uint32(len(c.variables)) {
		return arena.ErrInvalidSlot
	}
	return nil
}

// Root creates the source's initial all-unbounded node before unit assertion,
// the definition sweep, and queued propagation.
func (c *ArbitraryContext) Root() (*ArbitraryNode, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if c.root == nil {
		c.timestamp = 0
		c.root = &ArbitraryNode{
			context: c, lower: make([]*ArbitraryBound, len(c.variables)),
			upper: make([]*ArbitraryBound, len(c.variables)), conflictVar: math.MaxUint32,
		}
		c.nodes = append(c.nodes, c.root)
		c.pushLeafFront(c.root)
	}
	return c.root, nil
}

func (n *ArbitraryNode) Bounds(variable uint32) (lower, upper *ArbitraryBound, err error) {
	if n == nil || n.context == nil {
		return nil, nil, arena.ErrInvalidSlot
	}
	if err = n.context.checkNode(n, variable); err != nil {
		return nil, nil, err
	}
	return n.lower[variable], n.upper[variable], nil
}

func (n *ArbitraryNode) ConflictingBounds(variable uint32) (bool, error) {
	l, u, err := n.Bounds(variable)
	if err != nil || l == nil || u == nil {
		return false, err
	}
	m := n.context.numbers
	less, err := m.Less(&u.value, &l.value)
	if err != nil || less {
		return less, err
	}
	if !l.open && !u.open {
		return false, nil
	}
	return m.Equal(&u.value, &l.value)
}

func (n *ArbitraryNode) IsZero(variable uint32) (bool, error) {
	l, u, err := n.Bounds(variable)
	if err != nil || l == nil || u == nil || l.open || u.open {
		return false, err
	}
	lz, err := n.context.numbers.IsZero(&l.value)
	if err != nil || !lz {
		return false, err
	}
	return n.context.numbers.IsZero(&u.value)
}

func (n *ArbitraryNode) IsUpperZero(variable uint32) (bool, error) {
	_, u, err := n.Bounds(variable)
	if err != nil || u == nil || u.open {
		return false, err
	}
	return n.context.numbers.IsZero(&u.value)
}

// PublishBound follows mk_bound: an integer variable first gets ceil/floor,
// then a still-open integral endpoint moves one step in the directed mode.
// It records the owned numeral before any fallible copy so Close can recover
// a stopped construction.
func (c *ArbitraryContext) PublishBound(n *ArbitraryNode, variable uint32, value *numeral.ArbitraryFloat, lower, open, axiom bool) error {
	return c.publishBoundReason(n, variable, value, lower, open, axiom, nil)
}

func (c *ArbitraryContext) publishBoundReason(n *ArbitraryNode, variable uint32, value *numeral.ArbitraryFloat, lower, open, axiom bool, clause *ArbitraryClause) error {
	return c.publishBoundWithReason(n, variable, value, lower, open, axiom, clause, 0, false, true)
}

func (c *ArbitraryContext) publishBoundWithReason(n *ArbitraryNode, variable uint32, value *numeral.ArbitraryFloat, lower, open, axiom bool, clause *ArbitraryClause, reasonVariable uint32, definitionReason, enqueue bool) error {
	if err := c.checkNode(n, variable); err != nil {
		return err
	}
	if value == nil || n.conflictVar != math.MaxUint32 {
		return arena.ErrInvalidSlot
	}
	c.numBounds++
	b := &ArbitraryBound{context: c, variable: variable, lower: lower, open: open, axiom: axiom, reasonClause: clause, reasonVariable: reasonVariable, definitionReason: definitionReason}
	c.bounds = append(c.bounds, b)
	m := c.numbers
	if c.variables[variable] {
		integer, err := m.IsInteger(value)
		if err != nil {
			return err
		}
		if !integer {
			b.open = false
		}
		if lower {
			if err := m.Ceil(value, &b.value); err != nil {
				return err
			}
		} else if err := m.Floor(value, &b.value); err != nil {
			return err
		}
		if b.open {
			b.open = false
			m.SetRounding(!lower)
			if lower {
				if err := m.Increment(&b.value); err != nil {
					return err
				}
			} else if err := m.Decrement(&b.value); err != nil {
				return err
			}
		}
	} else if err := m.Copy(&b.value, value); err != nil {
		return err
	}
	b.timestamp = c.timestamp
	b.previous = n.trail
	n.trail = b
	if lower {
		n.lower[variable] = b
	} else {
		n.upper[variable] = b
	}
	if err := c.traceNewBound(b); err != nil {
		return err
	}
	conflict, err := n.ConflictingBounds(variable)
	if err != nil {
		return err
	}
	if conflict {
		if err := c.traceBoundConflict(n); err != nil {
			return err
		}
		n.conflictVar = variable
		c.numConflicts++
		c.removeLeaf(n)
	}
	c.timestamp++
	if c.timestamp == math.MaxUint64 {
		return ErrArbitraryContextTimestamp
	}
	if enqueue {
		c.queue = append(c.queue, b)
	}
	return nil
}

// AssertRootUnits checkpoints before each unit and resumes after a canceled
// checkpoint without replaying the completed prefix.
func (c *ArbitraryContext) AssertRootUnits() (*ArbitraryNode, error) {
	n, err := c.Root()
	if err != nil {
		return nil, err
	}
	for c.unitHead < len(c.unitClauses) && n.conflictVar == math.MaxUint32 {
		if err := sourceCheckpoint(c.limit, c.runtime, c.process, c.maxMemory, ErrArbitraryContextCanceled, c.cooperation); err != nil {
			return n, err
		}
		unit := c.unitClauses[c.unitHead]
		a := unit.atom
		if err := c.traceInitialUnit(a, unit.axiom); err != nil {
			return n, err
		}
		if err := c.PublishBound(n, a.variable, &a.value, a.lower, a.open, unit.axiom); err != nil {
			return n, err
		}
		c.unitHead++
	}
	if !c.initUnitBoundsTraced {
		if err := c.traceInitialBounds(n, "bounds after init\n", 1785); err != nil {
			return n, err
		}
		c.initUnitBoundsTraced = true
	}
	return n, nil
}
