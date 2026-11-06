package polynomial

// Source: src/math/polynomial/polynomial.cpp:2159-2214,3875,6865.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// Children are the actual published SOM owners. pending retains only a child
// whose construction or owner binding failed before native publication.
type PolynomialSOMVector struct {
	self                                            *PolynomialSOMVector
	storage, owner                                  *MultivariatePolynomialCore
	children                                        []*PolynomialSOMBuffer
	pending                                         *PolynomialSOMBuffer
	header                                          uint64
	cursor, closeCursor                             uint32
	sourceStage                                     uint8
	sourceCause                                     error
	sourceStarted, sourceFinalized, closing, closed bool
}

func NewPolynomialSOMVector(m *MultivariatePolynomialCore) (*PolynomialSOMVector, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	h := uint64(reflect.TypeOf(PolynomialSOMVector{}).Size())
	if e := m.integer.Budget().Reserve(h); e != nil {
		return nil, e
	}
	v := &PolynomialSOMVector{storage: m, header: h}
	v.self = v
	return v, nil
}
func (v *PolynomialSOMVector) ready() error {
	if v == nil || v.self != v {
		return ErrPolynomialOwner
	}
	if v.closed || v.closing || v.sourceStarted {
		return arena.ErrClosed
	}
	if v.pending != nil {
		return ErrPolynomialPending
	}
	return v.storage.check()
}
func (v *PolynomialSOMVector) Size() (uint32, error) {
	if e := v.ready(); e != nil {
		return 0, e
	}
	return uint32(len(v.children)), nil
}
func (v *PolynomialSOMVector) child(i uint32) (*PolynomialSOMBuffer, error) {
	if uint64(i) >= uint64(len(v.children)) {
		if polynomialSourceDebug {
			return nil, ErrPolynomialAssertion
		}
		return nil, arena.ErrBounds
	}
	b := v.children[i]
	if b == nil {
		return nil, ErrPolynomialOwner
	}
	return b, nil
}
func (v *PolynomialSOMVector) SetOwner(m *MultivariatePolynomialCore) error {
	if e := v.ready(); e != nil {
		return e
	}
	if m != v.storage {
		return ErrPolynomialOwner
	}
	if e := polynomialSOMPredicateSourceAssert(2187, "m_owner == owner || m_owner == 0", func() (bool, error) {
		return v.owner == m || v.owner == nil, nil
	}); e != nil {
		return e
	}
	if v.owner == nil {
		v.owner = m
		n := uint32(len(v.children))
		for i := uint32(0); i < n; i++ {
			b, e := v.child(i)
			if e != nil {
				return e
			}
			if e = b.SetOwner(m); e != nil {
				return e
			}
		}
	}
	return nil
}

func (v *PolynomialSOMVector) At(i uint32) (*PolynomialSOMBuffer, error) {
	if e := v.ready(); e != nil {
		return nil, e
	}
	n := uint32(len(v.children))
	requested := i + 1 // source unsigned ensure_capacity(idx+1) wraps first.
	for n < requested {
		// Actual alloc(child) and owner binding precede pointer-vector push.
		b, e := NewPolynomialSOMBuffer(v.storage)
		if b != nil {
			v.pending = b
		}
		if e != nil {
			return nil, e
		}
		if v.owner != nil {
			if e = b.SetOwner(v.owner); e != nil {
				return nil, e
			}
		}
		v.traceSOMChildAllocated(b)
		v.children = append(v.children, b)
		v.pending = nil
		n = uint32(len(v.children))
	}
	if e := polynomialSOMPredicateSourceAssert(2171, "m_buffers.size() >= sz", func() (bool, error) {
		return uint32(len(v.children)) >= requested, nil
	}); e != nil {
		return nil, e
	}
	return v.child(i)
}
func (v *PolynomialSOMVector) ResetN(n uint32) error {
	if e := v.ready(); e != nil {
		return e
	}
	old := uint32(len(v.children))
	if n > old {
		n = old
	}
	for i := uint32(0); i < n; i++ {
		b, e := v.child(i)
		if e != nil {
			return e
		}
		if e = b.Reset(); e != nil {
			return e
		}
	}
	return nil
}
func (v *PolynomialSOMVector) Reset() error {
	if e := v.ready(); e != nil {
		return e
	}
	n := uint32(len(v.children))
	return v.ResetN(n)
}
func (v *PolynomialSOMVector) SourceFinalize() error {
	if v == nil || v.self != v {
		return ErrPolynomialOwner
	}
	if v.closed || v.closing {
		return arena.ErrClosed
	}
	if v.sourceCause != nil {
		return v.sourceCause
	}
	if v.sourceFinalized {
		return nil
	}
	v.sourceStarted = true
	fail := func(e error) error { v.sourceCause = e; return e }
	if v.sourceStage == 0 {
		n := uint32(len(v.children))
		for v.cursor < n {
			b, e := v.child(v.cursor)
			if e != nil {
				return fail(e)
			}
			if e = b.SourceFinalize(); e != nil {
				return fail(e)
			}
			v.cursor++
		}
		v.sourceStage++
	}
	if v.sourceStage == 1 {
		v.sourceStage++
	}
	v.sourceFinalized = true
	return nil
}
func (v *PolynomialSOMVector) CloseChecked() error {
	if v == nil || v.self != v {
		return ErrPolynomialOwner
	}
	if v.closed {
		return nil
	}
	v.closing = true
	for v.closeCursor < uint32(len(v.children)) {
		b := v.children[v.closeCursor]
		if b != nil {
			if e := b.CloseChecked(); e != nil {
				return e
			}
			v.traceSOMChildDeallocated(b)
			v.children[v.closeCursor] = nil
		}
		v.closeCursor++
	}
	if v.pending != nil {
		if e := v.pending.CloseChecked(); e != nil {
			return e
		}
		v.pending = nil
	}
	v.storage.integer.Budget().Release(v.header)
	v.header = 0
	v.children = nil
	v.closed = true
	return nil
}
