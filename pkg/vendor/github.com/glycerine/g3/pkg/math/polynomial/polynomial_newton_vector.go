package polynomial

// Source: src/math/polynomial/polynomial.cpp:2914-2952.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// children contains active interpolators; orphans retains only real unjoined
// child resources left after successful source destruction for owning abort.
type PolynomialNewtonVector struct {
	self                              *PolynomialNewtonVector
	owner, storage                    *MultivariatePolynomialCore
	children                          []*PolynomialNewtonInterpolator
	header                            uint64
	cursor                            uint32
	closing, closed                   bool
	orphans                           []*PolynomialNewtonInterpolator
	sourceFinalizing, sourceFinalized bool
	sourceStage                       uint8
	sourceCause                       error
}

func NewPolynomialNewtonVector(m *MultivariatePolynomialCore) (*PolynomialNewtonVector, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	h := uint64(reflect.TypeOf(PolynomialNewtonVector{}).Size())
	if e := m.integer.Budget().Reserve(h); e != nil {
		return nil, e
	}
	v := &PolynomialNewtonVector{storage: m, header: h}
	v.self = v
	return v, nil
}
func (v *PolynomialNewtonVector) Size() (uint32, error) {
	if v == nil || v.self != v {
		return 0, ErrPolynomialOwner
	}
	if v.closed || v.closing || v.sourceFinalizing {
		return 0, arena.ErrClosed
	}
	return uint32(len(v.children)), nil
}

func (v *PolynomialNewtonVector) At(i uint32) (*PolynomialNewtonInterpolator, error) {
	if v == nil || v.self != v {
		return nil, ErrPolynomialOwner
	}
	if v.closed || v.closing || v.sourceFinalizing {
		return nil, arena.ErrClosed
	}
	if e := v.storage.check(); e != nil {
		return nil, e
	}
	if v.cursor != 0 || len(v.orphans) != 0 {
		return nil, ErrPolynomialPending
	}
	if v.owner == nil {
		if e := polynomialNewtonScalarAssert(2946, "m_imp", false); e != nil {
			return nil, e
		}
		return nil, ErrPolynomialOwner
	}
	for uint64(i) >= uint64(len(v.children)) {
		child, e := NewPolynomialNewtonInterpolator(v.owner)
		if e != nil {
			return nil, e
		}
		v.traceNewtonAllocated(child)
		v.children = append(v.children, child)
	}
	return v.children[i], nil
}
func (v *PolynomialNewtonVector) Flush() error {
	if v == nil || v.self != v {
		return ErrPolynomialOwner
	}
	if v.closed || v.closing || v.sourceFinalizing {
		return arena.ErrClosed
	}
	return v.flushMode(false)
}
func (v *PolynomialNewtonVector) flushMode(owning bool) error {
	if v == nil || v.self != v {
		return ErrPolynomialOwner
	}
	if v.closed {
		return arena.ErrClosed
	}
	for uint64(v.cursor) < uint64(len(v.children)) {
		child := v.children[v.cursor]
		if e := child.closeMode(owning, false); e != nil {
			return e
		}
		if !child.closed && child.header != 0 {
			// Source dealloc releases the heap header after the completed
			// destructor body. Only unfinished child resources stay orphaned.
			v.storage.integer.Budget().Release(child.header)
			child.header = 0
		}
		v.traceNewtonDeallocated(child)
		if !child.closed {
			v.orphans = append(v.orphans, child)
		}
		v.children[v.cursor] = nil
		v.cursor++
	}
	v.children = v.children[:0]
	v.cursor = 0
	return nil
}
func (v *PolynomialNewtonVector) Close() error {
	if v == nil || v.self != v {
		return ErrPolynomialOwner
	}
	if v.closed {
		return nil
	}
	v.closing = true
	if e := v.flushMode(true); e != nil {
		return e
	}
	// Source child destruction precedes abortion of real unjoined child roots.
	v.children = nil
	for i, child := range v.orphans {
		if child != nil {
			if e := child.Close(); e != nil {
				return e
			}
			v.orphans[i] = nil
		}
	}
	v.orphans = nil
	v.storage.integer.Budget().Release(v.header)
	v.header = 0
	v.closed = true
	return nil
}

// SetOwner follows the source nullable owner assignment. The Go allocation
// context is independent and remains the actual owner of the vector backing.
func (v *PolynomialNewtonVector) SetOwner(m *MultivariatePolynomialCore) error {
	if v == nil || v.self != v {
		return ErrPolynomialOwner
	}
	if v.closed || v.closing || v.sourceFinalizing {
		return arena.ErrClosed
	}
	if e := polynomialNewtonScalarAssert(2941, "m_imp == 0 || m_imp == owner", v.owner == nil || v.owner == m); e != nil {
		return e
	}
	if m != nil {
		if e := m.check(); e != nil {
			return e
		}
	}
	v.owner = m
	return nil
}
