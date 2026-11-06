package polynomial

// Source: src/math/polynomial/polynomial.cpp:6179-6232,6307-6344; src/util/vector.h.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// PolynomialVariablesView borrows the concrete caller's variable slice.
type PolynomialVariablesView struct{ values []uint32 }

func (v PolynomialVariablesView) At(i uint32) (uint32, error) {
	if v.values == nil {
		return 0, umemory.ErrRuntimeOwner
	}
	if uint64(i) >= uint64(len(v.values)) {
		return 0, arena.ErrBounds
	}
	return v.values[i], nil
}
func (v PolynomialVariablesView) IsNull() bool { return v.values == nil }
func variableEmbedded(d *PolynomialVariableDegrees, r *umemory.Runtime, complete bool) {
	*d = PolynomialVariableDegrees{runtime: r, constructed: complete}
	d.self = d
}
func variableGet(d *PolynomialVariableDegrees, x, def uint32) (uint32, error) {
	if uint64(x) >= uint64(len(d.values)) {
		return def, nil
	}
	return d.values[x], nil
}
func variableAt(d *PolynomialVariableDegrees, x uint32) (uint32, error) {
	if uint64(x) >= uint64(len(d.values)) {
		return 0, ErrPolynomialAssertion
	}
	return d.values[x], nil
}
func variablePut(d *PolynomialVariableDegrees, x, v uint32) error {
	if uint64(x) >= uint64(len(d.values)) {
		return ErrPolynomialAssertion
	}
	d.values[x] = v
	return nil
}

// The filled resize overload grows through ALL allocations with old SIZE,
// publishes requested SIZE once, then constructs its unsigned values ascending.
func variableResizeFilled(d *PolynomialVariableDegrees, n, fill uint32) error {
	old := len(d.values)
	if e := d.resize(n); e != nil {
		return e
	}
	for i := old; i < len(d.values); i++ {
		d.values[i] = fill
	}
	return nil
}
func variableSetFilled(d *PolynomialVariableDegrees, x, value, fill uint32) error {
	if x == ^uint32(0) {
		return arena.ErrBounds
	}
	if uint64(x) >= uint64(len(d.values)) {
		if e := variableResizeFilled(d, x+1, fill); e != nil {
			return e
		}
	}
	d.values[x] = value
	return nil
}
func variablePush(d *PolynomialVariableDegrees, value uint32) error {
	d.values = append(d.values, value)
	return nil
}
func variableReset(d *PolynomialVariableDegrees) error {
	d.values = d.values[:0]
	return nil
}
func variableView(d *PolynomialVariableDegrees) PolynomialVariablesView {
	return PolynomialVariablesView{values: d.values}
}

// Each member is a real uint-vector state embedded in one charged stable cold
// owner. Its header charge is zero; no per-vector Go owner allocation occurs.
type PolynomialMaxDegrees struct {
	self                                                *PolynomialMaxDegrees
	core                                                *MultivariatePolynomialCore
	runtime                                             *umemory.Runtime
	maxDegree, variables                                PolynomialVariableDegrees
	header                                              uint64
	constructed, closing, finalizing, finalized, closed bool
	sourceStage                                         uint8
	bodyCause, sourceCause                              error
}

func newPolynomialMaxDegrees(m *MultivariatePolynomialCore, r *umemory.Runtime, complete bool) (*PolynomialMaxDegrees, error) {
	if e := r.CheckOpen(); e != nil {
		return nil, e
	}
	h := uint64(reflect.TypeOf(PolynomialMaxDegrees{}).Size())
	if e := r.Budget().Reserve(h); e != nil {
		return nil, e
	}
	d := &PolynomialMaxDegrees{core: m, runtime: r, header: h, constructed: complete}
	d.self = d
	variableEmbedded(&d.maxDegree, r, complete)
	variableEmbedded(&d.variables, r, complete)
	return d, nil
}
func NewPolynomialMaxDegrees(m *MultivariatePolynomialCore) (*PolynomialMaxDegrees, error) {
	if m == nil {
		return nil, ErrPolynomialOwner
	}
	if e := m.check(); e != nil {
		return nil, e
	}
	return newPolynomialMaxDegrees(m, m.memory, true)
}
func (d *PolynomialMaxDegrees) identity() error {
	if d == nil || d.self != d {
		return ErrPolynomialOwner
	}
	if d.closed {
		return arena.ErrClosed
	}
	return d.runtime.CheckOpen()
}
func (d *PolynomialMaxDegrees) open() error {
	if e := d.identity(); e != nil {
		return e
	}
	if !d.constructed {
		return ErrPolynomialPending
	}
	if d.closing || d.finalizing {
		return arena.ErrClosed
	}
	return nil
}
func (d *PolynomialMaxDegrees) Init(p Polynomial) error {
	if e := d.open(); e != nil {
		return e
	}
	m := d.core
	if e := m.check(); e != nil {
		return e
	}
	n, e := m.readWord(p, 8)
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		term, e := m.monomialCore(p, i)
		if e != nil {
			return e
		}
		count, e := m.monomials.sizeCore(term)
		if e != nil {
			return e
		}
		for j := uint32(0); j < count; j++ {
			x, e := m.monomials.varCore(term, j)
			if e != nil {
				return e
			}
			k, e := m.monomials.degreeAtCore(term, j)
			if e != nil {
				return e
			}
			old, e := variableGet(&d.maxDegree, x, 0)
			if e != nil {
				return e
			}
			if k > old {
				if old == 0 {
					if e = variablePush(&d.variables, x); e != nil {
						d.bodyCause = e
						return e
					}
				}
				k, e = m.monomials.degreeAtCore(term, j)
				if e != nil {
					d.bodyCause = e
					return e
				}
				if e = variableSetFilled(&d.maxDegree, x, k, 0); e != nil {
					d.bodyCause = e
					return e
				}
			}
		}
	}
	return nil
}
func (d *PolynomialMaxDegrees) Reset() error {
	if e := d.open(); e != nil {
		return e
	}
	n, e := d.variables.size()
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		x, e := variableAt(&d.variables, i)
		if e != nil {
			return e
		}
		if e = variablePut(&d.maxDegree, x, 0); e != nil {
			return e
		}
	}
	return variableReset(&d.variables)
}
func (d *PolynomialMaxDegrees) Degree(x uint32) (uint32, error) {
	if e := d.open(); e != nil {
		return 0, e
	}
	return variableGet(&d.maxDegree, x, 0)
}
func (d *PolynomialMaxDegrees) NumVariables() (uint32, error) {
	if e := d.open(); e != nil {
		return 0, e
	}
	return d.variables.size()
}
func (d *PolynomialMaxDegrees) Vars() (PolynomialVariablesView, error) {
	if e := d.open(); e != nil {
		return PolynomialVariablesView{}, e
	}
	return variableView(&d.variables), nil
}

type PolynomialVariablePositions struct {
	self                                                *PolynomialVariablePositions
	runtime                                             *umemory.Runtime
	positions                                           PolynomialVariableDegrees
	header                                              uint64
	constructed, closing, finalizing, finalized, closed bool
	bodyCause, sourceCause                              error
}

func newPolynomialVariablePositions(r *umemory.Runtime, complete bool) (*PolynomialVariablePositions, error) {
	if e := r.CheckOpen(); e != nil {
		return nil, e
	}
	h := uint64(reflect.TypeOf(PolynomialVariablePositions{}).Size())
	if e := r.Budget().Reserve(h); e != nil {
		return nil, e
	}
	d := &PolynomialVariablePositions{runtime: r, header: h, constructed: complete}
	d.self = d
	variableEmbedded(&d.positions, r, complete)
	return d, nil
}
func NewPolynomialVariablePositions(r *umemory.Runtime) (*PolynomialVariablePositions, error) {
	return newPolynomialVariablePositions(r, true)
}
func (d *PolynomialVariablePositions) identity() error {
	if d == nil || d.self != d {
		return ErrPolynomialOwner
	}
	if d.closed {
		return arena.ErrClosed
	}
	return d.runtime.CheckOpen()
}
func (d *PolynomialVariablePositions) open() error {
	if e := d.identity(); e != nil {
		return e
	}
	if !d.constructed {
		return ErrPolynomialPending
	}
	if d.closing || d.finalizing {
		return arena.ErrClosed
	}
	return nil
}
func (d *PolynomialVariablePositions) Init(n uint32, xs PolynomialVariablesView) error {
	if e := d.open(); e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		if e := polynomialSubstitutionSourceAssert(6313, "m_pos.get(xs[i], UINT_MAX) == UINT_MAX", func() (bool, error) {
			x, e := xs.At(i)
			if e != nil {
				return false, e
			}
			old, e := variableGet(&d.positions, x, ^uint32(0))
			if e != nil {
				return false, e
			}
			return old == ^uint32(0), nil
		}); e != nil {
			return e
		}
		x, e := xs.At(i)
		if e != nil {
			return e
		}
		if e = variableSetFilled(&d.positions, x, i, ^uint32(0)); e != nil {
			d.bodyCause = e
			return e
		}
	}
	return nil
}
func (d *PolynomialVariablePositions) Reset(n uint32, xs PolynomialVariablesView) error {
	if e := d.open(); e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		if e := polynomialSubstitutionSourceAssert(6320, "m_pos.get(xs[i], UINT_MAX) != UINT_MAX", func() (bool, error) {
			x, e := xs.At(i)
			if e != nil {
				return false, e
			}
			old, e := variableGet(&d.positions, x, ^uint32(0))
			if e != nil {
				return false, e
			}
			return old != ^uint32(0), nil
		}); e != nil {
			return e
		}
		x, e := xs.At(i)
		if e != nil {
			return e
		}
		if e = variablePut(&d.positions, x, ^uint32(0)); e != nil {
			return e
		}
	}
	return nil
}
func (d *PolynomialVariablePositions) Position(x uint32) (uint32, error) {
	if e := d.open(); e != nil {
		return 0, e
	}
	return variableGet(&d.positions, x, ^uint32(0))
}
