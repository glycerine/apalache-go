package subpaving

// Source: src/math/subpaving/subpaving_t.h:146-237 and
// src/math/subpaving/subpaving_t_def.h:296-348, concrete config_hwf nodes.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// HardwareBound owns a scalar binary64 value. Node arrays retain pointers to
// bounds from the node or one of its ancestors.
type HardwareBound struct {
	value          numeral.HardwareFloat
	variable       uint32
	lower          bool
	open           bool
	reason         hardwareBoundReasonKind
	reasonVariable uint32
	reasonClause   *HardwareClause
	timestamp      uint64
	prev           *HardwareBound
}

type hardwareBoundReasonKind uint8

const (
	hardwareReasonUnknown hardwareBoundReasonKind = iota
	hardwareReasonAxiom
	hardwareReasonAssumption
	hardwareReasonVariableDefinition
	hardwareReasonClause
)

// HardwareNode uses copied Go slices for the retained parray bound views.
// A child's replacement of a bound never changes its parent's view.
type HardwareNode struct {
	self         *HardwareNode
	numbers      *numeral.HardwareFloatNumeralManager
	parent       *HardwareNode
	lower, upper []*HardwareBound
	owned        []*HardwareBound
	trail        *HardwareBound
	id           uint32
	depth        uint32
	children     uint32
	context      *HardwareContext
	leafPrev     *HardwareNode
	leafNext     *HardwareNode
	queuedLeaf   bool
	registered   bool
	conflictVar  uint32
	closed       bool
}

func NewHardwareNode(m *numeral.HardwareFloatNumeralManager, variables uint32, parent *HardwareNode, id uint32) (*HardwareNode, error) {
	if m == nil {
		return nil, arena.ErrInvalidSlot
	}
	if parent != nil {
		if err := parent.open(); err != nil {
			return nil, err
		}
		if parent.numbers != m || uint32(len(parent.lower)) != variables || parent.depth == math.MaxUint32 || parent.children == math.MaxUint32 {
			return nil, arena.ErrInvalidSlot
		}
	}
	n := &HardwareNode{numbers: m, parent: parent, lower: make([]*HardwareBound, variables), upper: make([]*HardwareBound, variables), id: id, conflictVar: math.MaxUint32}
	n.self = n
	if parent != nil {
		copy(n.lower, parent.lower)
		copy(n.upper, parent.upper)
		n.trail = parent.trail
		n.depth = parent.depth + 1
		n.conflictVar = parent.conflictVar
		n.context = parent.context
		parent.children++
	}
	return n, nil
}

func (n *HardwareNode) open() error {
	if n == nil || n.self != n || n.numbers == nil {
		return arena.ErrInvalidSlot
	}
	if n.closed {
		return arena.ErrClosed
	}
	return nil
}

func (n *HardwareNode) bound(variable uint32, lower bool) (*HardwareBound, error) {
	if err := n.open(); err != nil {
		return nil, err
	}
	if variable >= uint32(len(n.lower)) {
		return nil, arena.ErrInvalidSlot
	}
	if lower {
		return n.lower[variable], nil
	}
	return n.upper[variable], nil
}

// PushBound copies a context-selected scalar and updates only this node's
// trail and bound view. Relevance and conflict detection belong to the context.
func (n *HardwareNode) PushBound(variable uint32, value *numeral.HardwareFloat, lower, open bool) error {
	if err := n.open(); err != nil {
		return err
	}
	if variable >= uint32(len(n.lower)) {
		return arena.ErrInvalidSlot
	}
	b := &HardwareBound{variable: variable, lower: lower, open: open, prev: n.trail}
	if err := n.numbers.Set(&b.value, value); err != nil {
		return err
	}
	n.owned = append(n.owned, b)
	n.trail = b
	if lower {
		n.lower[variable] = b
	} else {
		n.upper[variable] = b
	}
	return nil
}

func (n *HardwareNode) Close() error {
	if err := n.open(); err != nil {
		if n != nil && n.self == n && n.closed {
			return nil
		}
		return err
	}
	if n.children != 0 {
		return arena.ErrInvalidSlot
	}
	if n.context != nil {
		n.context.removeLeaf(n)
	}
	parent := n.parent
	n.owned = nil
	n.lower = nil
	n.upper = nil
	n.trail = nil
	n.parent = nil
	n.closed = true
	if n.registered {
		n.context.activeNodes--
	}
	if parent != nil {
		parent.children--
	}
	return nil
}

// CreateRootNode allocates the context's initial bound views after variables
// and definitions are registered. Unit assertions and propagation follow.
func (c *HardwareContext) CreateRootNode() (*HardwareNode, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if c.root != nil {
		return nil, arena.ErrInvalidSlot
	}
	n, err := NewHardwareNode(c.numbers, uint32(len(c.isInt)), nil, 0)
	if err != nil {
		return nil, err
	}
	n.context = c
	n.registered = true
	c.root = n
	c.nodes = append(c.nodes, n)
	c.activeNodes++
	c.nextNodeID = 1
	c.pushLeafFront(n)
	return n, nil
}

// NewChildNode copies the selected parent's bound views and trail.
func (c *HardwareContext) NewChildNode(parent *HardwareNode) (*HardwareNode, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if c.root == nil || parent == nil || parent.context != c || c.nextNodeID == math.MaxUint32 {
		return nil, arena.ErrInvalidSlot
	}
	n, err := NewHardwareNode(c.numbers, uint32(len(c.isInt)), parent, c.nextNodeID)
	if err != nil {
		return nil, err
	}
	c.nextNodeID++
	n.registered = true
	c.nodes = append(c.nodes, n)
	c.activeNodes++
	c.pushLeafFront(n)
	return n, nil
}
