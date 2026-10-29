package polynomial

// Source: src/math/polynomial/polynomial.cpp:449-505; src/math/polynomial/polynomial.h.
// Concrete translation of src/math/polynomial/polynomial.cpp:449-505 monomial2pos.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// MonomialPositions borrows its manager. It maps stable monomial IDs to
// positions with ordinary Go storage; neither monomials nor the manager are
// retained. The borrower must be closed before its manager.
type MonomialPositions struct {
	self       *MonomialPositions
	manager    *MonomialManager
	values     []uint32
	header     uint64
	finalizing bool
	closed     bool
}

func NewMonomialPositions(m *MonomialManager) (*MonomialPositions, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(MonomialPositions{}).Size())
	if e := m.config.Budget.Reserve(n); e != nil {
		return nil, e
	}
	p := &MonomialPositions{manager: m, header: n}
	p.self = p
	return p, nil
}
func (p *MonomialPositions) identity() error {
	if p == nil || p.self != p {
		return ErrMonomialOwner
	}
	if p.closed {
		return arena.ErrClosed
	}
	return nil
}
func (p *MonomialPositions) check() error {
	if e := p.identity(); e != nil {
		return e
	}
	if p.finalizing {
		return ErrMonomialPending
	}
	return p.manager.check()
}
func (p *MonomialPositions) id(a Monomial) (uint32, error) {
	l, e := p.manager.location(a)
	if e != nil {
		return 0, e
	}
	// monomial::id reads this word only, not RC/degree/size/hash.
	return p.manager.allocator.ReadUint32(l.raw, 4)
}
func (p *MonomialPositions) size() (uint32, error) {
	if uint64(len(p.values)) > uint64(^uint32(0)) {
		return 0, arena.ErrBounds
	}
	return uint32(len(p.values)), nil
}
func (p *MonomialPositions) reserve(n uint32) error {
	if uint64(n) > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	old := len(p.values)
	next := int(n)
	if next <= old {
		return nil
	}
	p.values = append(p.values, make([]uint32, next-old)...)
	for i := old; i < next; i++ {
		p.values[i] = ^uint32(0)
	}
	return nil
}
func (p *MonomialPositions) read(id uint32) (uint32, error) {
	if uint64(id) >= uint64(len(p.values)) {
		if polynomialSourceDebug {
			return 0, ErrMonomialAssertion
		}
		return 0, arena.ErrBounds
	}
	return p.values[id], nil
}
func (p *MonomialPositions) Get(a Monomial) (uint32, error) {
	if e := p.check(); e != nil {
		return 0, e
	}
	id, e := p.id(a)
	if e != nil {
		return 0, e
	}
	if e = p.reserve(id + 1); e != nil {
		return 0, e
	}
	return p.read(id)
}
func (p *MonomialPositions) Reset(a Monomial) error {
	if e := p.check(); e != nil {
		return e
	}
	id, e := p.id(a)
	if e != nil {
		return e
	}
	if e = p.resetIndexAssert(id); e != nil {
		return e
	}
	if uint64(id) >= uint64(len(p.values)) {
		return arena.ErrBounds
	}
	p.values[id] = ^uint32(0)
	return nil
}
func (p *MonomialPositions) Set(a Monomial, pos uint32) error {
	if e := p.check(); e != nil {
		return e
	}
	id, e := p.id(a)
	if e != nil {
		return e
	}
	if e = p.reserve(id + 1); e != nil {
		return e
	}
	if e = p.setEmptyAssert(id); e != nil {
		return e
	}
	if uint64(id) >= uint64(len(p.values)) {
		return arena.ErrBounds
	}
	p.values[id] = pos
	return nil
}

func (p *MonomialPositions) releaseValues() {
	p.values = nil
}
func (p *MonomialPositions) SourceFinalize() error {
	if e := p.identity(); e != nil {
		return e
	}
	p.finalizing = true
	p.releaseValues()
	return nil
}
func (p *MonomialPositions) CloseChecked() error {
	if p == nil || p.self != p {
		return ErrMonomialOwner
	}
	if p.closed {
		return nil
	}
	p.finalizing = true
	p.releaseValues()
	p.manager.config.Budget.Release(p.header)
	p.header = 0
	p.closed = true
	return nil
}
