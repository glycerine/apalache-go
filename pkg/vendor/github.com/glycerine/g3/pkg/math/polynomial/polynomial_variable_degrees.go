package polynomial

// Source: src/math/polynomial/polynomial.h:62-69; src/util/vector.h.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// PolynomialVariableDegrees stores the source var2degree values in ordinary
// Go storage. Runtime remains the borrowed lifetime owner for the surrounding
// polynomial subsystem; it does not own the slice backing.
type PolynomialVariableDegrees struct {
	collecting                                 bool
	self                                       *PolynomialVariableDegrees
	runtime                                    *umemory.Runtime
	values                                     []uint32
	header                                     uint64
	constructed, finalizing, finalized, closed bool
}

func NewPolynomialVariableDegrees(r *umemory.Runtime) (*PolynomialVariableDegrees, error) {
	if e := r.CheckOpen(); e != nil {
		return nil, e
	}
	h := uint64(reflect.TypeOf(PolynomialVariableDegrees{}).Size())
	if e := r.Budget().Reserve(h); e != nil {
		return nil, e
	}
	d := &PolynomialVariableDegrees{runtime: r, header: h, constructed: true}
	d.self = d
	return d, nil
}
func (d *PolynomialVariableDegrees) identity() error {
	if d == nil || d.self != d {
		return ErrPolynomialOwner
	}
	if d.closed {
		return arena.ErrClosed
	}
	return d.runtime.CheckOpen()
}
func (d *PolynomialVariableDegrees) open() error {
	if d != nil && d.self == d && d.collecting {
		return ErrPolynomialPending
	}
	if e := d.identity(); e != nil {
		return e
	}
	if !d.constructed {
		return ErrPolynomialPending
	}
	if d.finalizing {
		return arena.ErrClosed
	}
	return nil
}
func (d *PolynomialVariableDegrees) size() (uint32, error) {
	if uint64(len(d.values)) > uint64(^uint32(0)) {
		return 0, arena.ErrBounds
	}
	return uint32(len(d.values)), nil
}
func (d *PolynomialVariableDegrees) resize(n uint32) error {
	if uint64(n) > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	if int(n) <= len(d.values) {
		d.values = d.values[:n]
	} else {
		d.values = append(d.values, make([]uint32, int(n)-len(d.values))...)
	}
	return nil
}
func (d *PolynomialVariableDegrees) SetDegree(x, degree uint32) error {
	if e := d.open(); e != nil {
		return e
	}
	if x == ^uint32(0) {
		return arena.ErrBounds
	}
	if uint64(x) >= uint64(len(d.values)) {
		if e := d.resize(x + 1); e != nil {
			return e
		}
	}
	d.values[x] = degree
	return nil
}
func (d *PolynomialVariableDegrees) Degree(x uint32) (uint32, error) {
	if e := d.open(); e != nil {
		return 0, e
	}
	if uint64(x) >= uint64(len(d.values)) {
		return 0, nil
	}
	return d.values[x], nil
}
func (d *PolynomialVariableDegrees) SourceFinalize() error {
	if d != nil && d.self == d && d.collecting {
		return ErrPolynomialPending
	}
	if e := d.identity(); e != nil {
		return e
	}
	if !d.constructed || d.finalized {
		return nil
	}
	d.finalizing = true
	d.values = nil
	d.finalized = true
	return nil
}
func (d *PolynomialVariableDegrees) CloseChecked() error {
	if d != nil && d.self == d && d.collecting {
		return ErrPolynomialPending
	}
	if d == nil || d.self != d {
		return ErrPolynomialOwner
	}
	if d.closed {
		return nil
	}
	d.finalizing = true
	d.values = nil
	d.runtime.Budget().Release(d.header)
	d.header = 0
	d.closed = true
	return nil
}
