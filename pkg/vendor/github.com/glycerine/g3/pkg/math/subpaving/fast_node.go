package subpaving

// Source: src/math/subpaving/subpaving_t.h:146-237 and
// src/math/subpaving/subpaving_t_def.h:296-348, concrete config_mpff.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// FastBound owns its scoped numeral. Node bound arrays borrow these values;
// a child can retain bounds created by its parent until it closes.
type fastBoundReasonKind uint8

const (
	fastReasonUnknown fastBoundReasonKind = iota
	fastReasonAxiom
	fastReasonAssumption
	fastReasonClause
	fastReasonVariableDefinition
)

type fastBoundReason struct {
	kind     fastBoundReasonKind
	clause   *FastClause // borrowed source justification pointer
	variable uint32
}

type FastBound struct {
	owner     *numeral.ScopedFastFloat
	value     *numeral.FastFloat
	variable  uint32
	lower     bool
	open      bool
	decision  bool
	reason    fastBoundReason
	timestamp uint64
	prev      *FastBound
}

// FastNode retains the concrete per-variable bounds needed by constant
// interval views. The enclosing propagation context is a separate port.
type FastNode struct {
	self         *FastNode
	numbers      *numeral.FastFloatManager
	parent       *FastNode
	lower, upper []*FastBound
	owned        []*FastBound
	trail        *FastBound
	children     uint32
	id           uint32
	depth        uint32
	queuedLeaf   bool
	context      *FastContext
	conflictVar  uint32
	inconsistent bool
	closing      bool
	closed       bool
}

func NewFastNode(m *numeral.FastFloatManager, variables uint32, parent *FastNode) (*FastNode, error) {
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
	n := &FastNode{numbers: m, parent: parent, lower: make([]*FastBound, variables), upper: make([]*FastBound, variables)}
	n.self = n
	if parent != nil {
		copy(n.lower, parent.lower)
		copy(n.upper, parent.upper)
		n.trail = parent.trail
		n.context = parent.context
		n.depth = parent.depth + 1
		n.conflictVar = parent.conflictVar
		n.inconsistent = parent.inconsistent
		parent.children++
	}
	return n, nil
}

func (n *FastNode) open() error {
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

func (n *FastNode) bound(variable uint32, lower bool) (*FastBound, error) {
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

// PushBound records a bound already selected by the context. Relevance,
// conflict detection and normalization belong to that context.
func (n *FastNode) PushBound(variable uint32, value *numeral.FastFloat, lower, open bool) error {
	if err := n.open(); err != nil {
		return err
	}
	if variable >= uint32(len(n.lower)) {
		return arena.ErrInvalidSlot
	}
	owner, err := numeral.NewScopedFastFloat(n.numbers)
	if err != nil {
		return err
	}
	v, err := owner.Get()
	if err != nil {
		return errors.Join(err, owner.Close())
	}
	if err := n.numbers.Copy(v, value); err != nil {
		return errors.Join(err, owner.Close())
	}
	b := &FastBound{owner: owner, value: v, variable: variable, lower: lower, open: open, prev: n.trail}
	n.owned = append(n.owned, b)
	n.trail = b
	if lower {
		n.lower[variable] = b
	} else {
		n.upper[variable] = b
	}
	return nil
}

func (n *FastNode) Close() error {
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
		if err := b.owner.Close(); err != nil {
			return err
		}
		n.trail = b.prev
		n.owned[i] = nil
		n.owned = n.owned[:i]
	}
	n.closed = true
	if n.context != nil && n.queuedLeaf {
		n.context.removeLeaf(n)
	}
	if n.parent != nil {
		n.parent.children--
		if n.context != nil {
			for i, child := range n.context.nodes {
				if child == n {
					copy(n.context.nodes[i:], n.context.nodes[i+1:])
					last := len(n.context.nodes) - 1
					n.context.nodes[last] = nil
					n.context.nodes = n.context.nodes[:last]
					break
				}
			}
		}
	}
	return nil
}
