package polynomial

// Source: src/util/scoped_ptr_vector.h; src/math/polynomial/polynomial.cpp:4071.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// data contains source-visible pointers. children retains published child
// contexts, including completed source destructors needing owning completion.
type PolynomialSkeletonVector struct {
	self                                            *PolynomialSkeletonVector
	owner                                           *MultivariatePolynomialCore
	data                                            []*PolynomialSkeleton
	children                                        []*PolynomialSkeleton
	header                                          uint64
	sourceCause                                     error
	cursor, closeCursor                             uint32
	sourceStarted, sourceFinalized, closing, closed bool
}

func NewPolynomialSkeletonVector(m *MultivariatePolynomialCore) (*PolynomialSkeletonVector, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	h := uint64(reflect.TypeOf(PolynomialSkeletonVector{}).Size())
	if e := m.integer.Budget().Reserve(h); e != nil {
		return nil, e
	}
	v := &PolynomialSkeletonVector{owner: m, header: h}
	v.self = v
	return v, nil
}
func (v *PolynomialSkeletonVector) ready() error {
	if v == nil || v.self != v {
		return ErrPolynomialOwner
	}
	if v.closed || v.closing || v.sourceStarted {
		return arena.ErrClosed
	}
	if v.sourceCause != nil {
		return v.sourceCause
	}
	return v.owner.check()
}
func (v *PolynomialSkeletonVector) Size() (uint32, error) {
	if e := v.ready(); e != nil {
		return 0, e
	}
	return uint32(len(v.data)), nil
}
func (v *PolynomialSkeletonVector) pointer(i uint32) (*PolynomialSkeleton, error) {
	if e := polynomialSkeletonVectorIndex(i, len(v.data)); e != nil {
		return nil, e
	}
	return v.data[i], nil
}
func (v *PolynomialSkeletonVector) Get(i uint32) (*PolynomialSkeleton, error) {
	if e := v.ready(); e != nil {
		return nil, e
	}
	return v.pointer(i)
}

func (v *PolynomialSkeletonVector) prepare(p *PolynomialSkeleton) error {
	if p == nil {
		return nil
	}
	if p.self != p || p.owner != v.owner {
		return ErrPolynomialOwner
	}
	return p.ready()
}
func (v *PolynomialSkeletonVector) Push(p *PolynomialSkeleton) error {
	if e := v.ready(); e != nil {
		return e
	}
	if uint64(len(v.data)) == uint64(^uint32(0)) {
		return arena.ErrBounds
	}
	if e := v.prepare(p); e != nil {
		return e
	}
	v.data = append(v.data, p)
	if p != nil {
		v.children = append(v.children, p)
	}
	return nil
}
func (v *PolynomialSkeletonVector) deleteChild(p *PolynomialSkeleton, line int) error {
	if p == nil {
		return nil
	}
	if e := p.SourceFinalize(); e != nil {
		v.sourceCause = e
		return e
	}
	if p.memoryTraceLive {
		p.traceGCDSkeletonDeallocated(line)
		p.memoryTraceLive = false
	}
	return nil
}
func (v *PolynomialSkeletonVector) Set(i uint32, p *PolynomialSkeleton) error {
	if e := v.ready(); e != nil {
		return e
	}
	old, e := v.pointer(i)
	if e != nil {
		return e
	}
	if old == p {
		return nil
	}
	// Source destruction of old precedes validation and publication of new.
	if e = v.deleteChild(old, 38); e != nil {
		return e
	}
	e = v.prepare(p)
	if e != nil {
		v.sourceCause = e
		return e
	}
	v.data[i] = p
	if p != nil {
		v.children = append(v.children, p)
	}
	return nil
}
func (v *PolynomialSkeletonVector) Pop() error {
	if e := v.ready(); e != nil {
		return e
	}
	n := uint32(len(v.data))
	if n == 0 {
		if e := polynomialSkeletonScalarAssert("src/util/scoped_ptr_vector.h", 33, "!empty()", false); e != nil {
			return e
		}
		return arena.ErrBounds
	}
	if e := v.Set(n-1, nil); e != nil {
		return e
	}
	v.data = v.data[:n-1]
	return nil
}
func (v *PolynomialSkeletonVector) Resize(n uint32) error {
	if e := v.ready(); e != nil {
		return e
	}
	old := uint32(len(v.data))
	if n < old {
		for i := old; i > n; {
			i--
			p, e := v.pointer(i)
			if e != nil {
				return e
			}
			if e = v.deleteChild(p, 46); e != nil {
				return e
			}
		}
		v.data = v.data[:n]
		return nil
	}
	for i := old; i < n; i++ {
		if e := v.Push(nil); e != nil {
			return e
		}
	}
	return nil
}
func (v *PolynomialSkeletonVector) resetSource() error {
	n := uint32(len(v.data))
	for v.cursor < n {
		p, e := v.pointer(v.cursor)
		if e != nil {
			return e
		}
		if e = v.deleteChild(p, 171); e != nil {
			return e
		}
		v.cursor++
	}
	v.data = v.data[:0]
	v.cursor = 0
	return nil
}
func (v *PolynomialSkeletonVector) Reset() error {
	if e := v.ready(); e != nil {
		return e
	}
	return v.resetSource()
}
func (v *PolynomialSkeletonVector) SourceFinalize() error {
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
	if e := v.resetSource(); e != nil {
		v.sourceCause = e
		return e
	}
	v.data = nil
	v.sourceFinalized = true
	return nil
}
func (v *PolynomialSkeletonVector) CloseChecked() error {
	if v == nil || v.self != v {
		return ErrPolynomialOwner
	}
	if v.closed {
		return nil
	}
	v.closing = true
	// These are only actual pointer publications and retained stopped source
	// destructors. No caller registry or unpublished borrowed pointer is scanned.
	for v.closeCursor < uint32(len(v.children)) {
		p := v.children[v.closeCursor]
		if p != nil {
			if e := p.CloseChecked(); e != nil {
				return e
			}
			if p.memoryTraceLive && p.sourceCause == nil {
				p.traceGCDSkeletonDeallocated(171)
				p.memoryTraceLive = false
			}
		}
		v.children[v.closeCursor] = nil
		v.closeCursor++
	}
	v.data = nil
	v.owner.integer.Budget().Release(v.header)
	v.children = nil
	v.header = 0
	v.closed = true
	return nil
}
