package subpaving

// Source: src/math/subpaving/subpaving_t.h:146-237 and
// src/math/subpaving/subpaving_t_def.h:296-348, concrete config_mpfx.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// FixedBound owns its numeral. A node's lower/upper arrays borrow bounds;
// each bound's creating node retains ownership until that node closes.
type fixedBoundReasonKind uint8

const (
	fixedReasonUnknown fixedBoundReasonKind = iota
	fixedReasonAxiom
	fixedReasonVariableDefinition
	fixedReasonAssumption
	fixedReasonClause
)

type fixedBoundReason struct {
	kind     fixedBoundReasonKind
	variable uint32
	clause   *FixedClause
}

type FixedBound struct {
	value     *numeral.FixedPoint
	variable  uint32
	lower     bool
	open      bool
	decision  bool
	reason    fixedBoundReason
	prev      *FixedBound
	timestamp uint64
}

// FixedNode keeps the source's per-variable bound views and shared parent
// bounds. PushBound receives a bound already selected by the context; bound
// relevance, normalization and conflict propagation belong to that context.
type FixedNode struct {
	self         *FixedNode
	numbers      *numeral.FixedPointManager
	parent       *FixedNode
	lower, upper []*FixedBound
	owned        []*FixedBound
	trail        *FixedBound
	children     uint32
	context      *FixedContext
	id           uint32
	conflictVar  uint32
	inconsistent bool
	closing      bool
	closed       bool
}

func NewFixedNode(m *numeral.FixedPointManager, variables uint32, parent *FixedNode) (*FixedNode, error) {
	if m == nil {
		return nil, arena.ErrInvalidSlot
	}
	if parent != nil {
		if err := parent.open(); err != nil {
			return nil, err
		}
		if parent.numbers != m || uint32(len(parent.lower)) != variables {
			return nil, arena.ErrInvalidSlot
		}
	}
	n := &FixedNode{numbers: m, parent: parent, lower: make([]*FixedBound, variables), upper: make([]*FixedBound, variables)}
	n.self = n
	if parent != nil {
		copy(n.lower, parent.lower)
		copy(n.upper, parent.upper)
		n.trail = parent.trail
		n.context = parent.context
		if n.context != nil {
			n.id = n.context.takeNodeID()
			n.context.numNodes++
			n.context.nodes = append(n.context.nodes, n)
		}
		n.conflictVar = parent.conflictVar
		n.inconsistent = parent.inconsistent
		parent.children++
	}
	return n, nil
}

func (n *FixedNode) open() error {
	if n == nil || n.self != n || n.numbers == nil {
		return arena.ErrInvalidSlot
	}
	if n.closed {
		return arena.ErrClosed
	}
	if n.closing {
		return arena.ErrClosed
	}
	return nil
}

func (n *FixedNode) bound(variable uint32, lower bool) (*FixedBound, error) {
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

func (n *FixedNode) PushBound(variable uint32, value *numeral.FixedPoint, lower, open bool) error {
	if err := n.open(); err != nil {
		return err
	}
	if variable >= uint32(len(n.lower)) {
		return arena.ErrInvalidSlot
	}
	v, err := n.numbers.New()
	if err != nil {
		return err
	}
	if err = n.numbers.Copy(v, value); err != nil {
		return errors.Join(err, v.Close())
	}
	b := &FixedBound{value: v, variable: variable, lower: lower, open: open, prev: n.trail}
	n.owned = append(n.owned, b)
	n.trail = b
	if lower {
		n.lower[variable] = b
	} else {
		n.upper[variable] = b
	}
	return nil
}

func (n *FixedNode) Close() error {
	if n == nil || n.self != n || n.numbers == nil {
		return arena.ErrInvalidSlot
	}
	if n.closed {
		return nil
	}
	if n.children != 0 {
		return arena.ErrInvalidSlot
	}
	n.closing = true
	for len(n.owned) != 0 {
		i := len(n.owned) - 1
		b := n.owned[i]
		if err := b.value.Close(); err != nil {
			return err
		}
		n.trail = b.prev
		n.owned[i] = nil
		n.owned = n.owned[:i]
	}
	n.closed = true
	if n.context != nil {
		n.context.numNodes--
		n.context.freeNodeIDs = append(n.context.freeNodeIDs, n.id)
	}
	if n.parent != nil {
		n.parent.children--
	}
	return nil
}
