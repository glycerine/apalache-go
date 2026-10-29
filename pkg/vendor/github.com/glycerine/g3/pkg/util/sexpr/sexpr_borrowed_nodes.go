package sexpr

// Source: src/util/sexpr.h/.cpp: borrowed nodes in synchronous source traversals.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	// SexprBorrowedNodes retains the manager lock during a caller's traversal.
	// Records carry the borrow generation, so a node from an earlier traversal
	// cannot acquire authority in a later callback.
)

import "github.com/glycerine/g3/pkg/arena"

type SexprBorrowedNodes struct {
	self       *SexprBorrowedNodes
	manager    *SexprManager
	generation uint64
	active     bool
}
type SexprBorrowedNode struct {
	Kind              SexprKind
	Line, Pos, Length uint32
	Symbol            usymbol.Symbol
	Numeral           *unumeral.OwnedRational
	borrow            *SexprBorrowedNodes
	generation        uint64
	handle            SexprHandle
}

func (b *SexprBorrowedNodes) Valid() bool {
	return b != nil && b.self == b && b.active && b.manager != nil && b == &b.manager.borrowedNodes
}
func (m *SexprManager) WithBorrowedNodes(body func(*SexprBorrowedNodes) error) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if body == nil {
		return arena.ErrConfig
	}
	b := &m.borrowedNodes
	if b.generation == ^uint64(0) {
		return arena.ErrBounds
	}
	*b = SexprBorrowedNodes{self: b, manager: m, generation: b.generation + 1, active: true}
	defer func() { b.active = false }()
	return body(b)
}
func (b *SexprBorrowedNodes) Record(h SexprHandle) (SexprBorrowedNode, error) {
	if !b.Valid() {
		return SexprBorrowedNode{}, ErrSexprOwner
	}
	r, e := b.manager.record(h)
	if e != nil {
		return SexprBorrowedNode{}, e
	}
	return SexprBorrowedNode{Kind: r.Kind, Line: r.line, Pos: r.pos, Length: r.length, Symbol: r.symbol, Numeral: &r.Numeral, borrow: b, generation: b.generation, handle: h}, nil
}
func (b *SexprBorrowedNodes) Child(n SexprBorrowedNode, i uint32) (SexprHandle, error) {
	if !b.Valid() || n.borrow != b || n.generation != b.generation {
		return SexprHandle{}, ErrSexprOwner
	}
	r, e := b.manager.record(n.handle)
	if e != nil {
		return SexprHandle{}, e
	}
	return b.manager.child(r, i)
}

// Configuration returns the actual borrowed services and shared arena budget.
func (m *SexprManager) Configuration() SexprConfig {
	if m == nil || m.self != m {
		return SexprConfig{}
	}
	return m.Config
}
