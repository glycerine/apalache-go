package polynomial

// Factorization subset degrees use native boolean slices.

import (
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

type upolyFactorDegreeState struct{ values []bool }

func (s *upolyFactorDegreeState) close() { s.values = nil }
func (s *upolyFactorDegreeState) get(k uint32) (bool, error) {
	if polynomialSourceDebug && uint64(k) >= uint64(len(s.values)) {
		return false, ErrUPolynomialAssertion
	}
	if uint64(k) >= uint64(len(s.values)) {
		return false, arena.ErrBounds
	}
	return s.values[k], nil
}
func (s *upolyFactorDegreeState) set(k uint32, v bool) error {
	if polynomialSourceDebug && uint64(k) >= uint64(len(s.values)) {
		return ErrUPolynomialAssertion
	}
	if uint64(k) >= uint64(len(s.values)) {
		return arena.ErrBounds
	}
	s.values[k] = v
	return nil
}
func (s *upolyFactorDegreeState) intersect(other *upolyFactorDegreeState) error {
	for i := range s.values {
		s.values[i] = s.values[i] && i < len(other.values) && other.values[i]
	}
	return nil
}
func (s *upolyFactorDegreeState) init(fs *UPolynomialFactors) error {
	if uint64(len(s.values)) >= math.MaxUint32 || len(s.values) == int(^uint(0)>>1) {
		return arena.ErrBounds
	}
	s.values = append(s.values, true)
	for i := uint32(0); i < fs.count; i++ {
		degree := upolyStateDegree(&fs.factors[i])
		mult, e := fs.multiplicity(i)
		if e != nil {
			return e
		}
		for k := uint32(0); k < mult; k++ {
			if e = s.includeDegree(degree); e != nil {
				return e
			}
		}
	}
	enabled, e := factorHeaderAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled {
		zero, e := s.get(0)
		if e != nil {
			return e
		}
		total := false
		if zero {
			total, e = s.get(fs.totalDegree)
			if e != nil {
				return e
			}
		}
		if !zero || !total {
			if e := factorHeaderAssertionFailure(84, "in_set(0) && in_set(factors.get_degree())"); e != nil {
				return e
			}
		}
	}
	return nil
}

// Descending subset-sum update keeps every original degree and also includes
// that degree plus the new factor's degree, without reusing this factor twice.
func (s *upolyFactorDegreeState) includeDegree(degree uint32) error {
	old := len(s.values)
	n := uint64(old) + uint64(degree)
	if n > math.MaxUint32 || n > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	s.values = append(s.values, make([]bool, int(degree))...)
	for i := old - 1; i >= 0; i-- {
		if s.values[i] {
			s.values[i+int(degree)] = true
		}
	}
	return nil
}
func (s *upolyFactorDegreeState) trivial() (bool, error) {
	for i := int32(1); i < int32(len(s.values))-1; i++ {
		if s.values[i] {
			return false, nil
		}
	}
	return true, nil
}

type UPolynomialDegreeSet struct {
	self       *UPolynomialDegreeSet
	budget     *arena.Budget
	state      upolyFactorDegreeState
	header     uint64
	closed     bool
	displaying bool
}

// WordBytes stays zero. CapacityWords reports logical groups of 32 degrees
// within native flag capacity; it does not describe packed word storage.
type UPolynomialDegreeSetStats struct {
	OwnerBytes, WordBytes uint64
	Bits, CapacityWords   uint32
}

func NewUPolynomialDegreeSet(fs *UPolynomialFactors) (*UPolynomialDegreeSet, error) {
	if e := fs.check(); e != nil {
		return nil, e
	}
	b := fs.manager.config.Integer.Budget()
	n := uint64(reflect.TypeOf(UPolynomialDegreeSet{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	d := &UPolynomialDegreeSet{budget: b, header: n}
	d.self = d
	if e := d.state.init(fs); e != nil {
		d.state.close()
		b.Release(n)
		return nil, e
	}
	return d, nil
}
func (d *UPolynomialDegreeSet) check() error {
	if d == nil || d.self != d {
		return ErrUPolynomialOwner
	}
	if d.closed {
		return arena.ErrClosed
	}
	return nil
}
func (d *UPolynomialDegreeSet) checkMutable() error {
	if e := d.check(); e != nil {
		return e
	}
	if d.displaying {
		return ErrUPolynomialCleanupPending
	}
	return nil
}
func (d *UPolynomialDegreeSet) MaxDegree() (uint32, error) {
	if e := d.check(); e != nil {
		return 0, e
	}
	return uint32(len(d.state.values)) - 1, nil
}
func (d *UPolynomialDegreeSet) InSet(k uint32) (bool, error) {
	if e := d.check(); e != nil {
		return false, e
	}
	return d.state.get(k)
}
func (d *UPolynomialDegreeSet) IsTrivial() (bool, error) {
	if e := d.check(); e != nil {
		return false, e
	}
	return d.state.trivial()
}
func (d *UPolynomialDegreeSet) Remove(k uint32) error {
	if e := d.checkMutable(); e != nil {
		return e
	}
	return d.state.set(k, false)
}
func (d *UPolynomialDegreeSet) Intersect(other *UPolynomialDegreeSet) error {
	if e := d.checkMutable(); e != nil {
		return e
	}
	if e := other.check(); e != nil {
		return e
	}
	return d.state.intersect(&other.state)
}
func (d *UPolynomialDegreeSet) Swap(other *UPolynomialDegreeSet) error {
	if e := d.checkMutable(); e != nil {
		return e
	}
	if e := other.checkMutable(); e != nil {
		return e
	}
	d.state, other.state = other.state, d.state
	return nil
}
func (d *UPolynomialDegreeSet) Close() error {
	if e := d.checkMutable(); e != nil {
		return e
	}
	d.state.close()
	d.budget.Release(d.header)
	d.closed = true
	return nil
}
func (d *UPolynomialDegreeSet) Stats() UPolynomialDegreeSetStats {
	if d == nil || d.self != d || d.closed {
		return UPolynomialDegreeSetStats{}
	}
	return UPolynomialDegreeSetStats{OwnerBytes: d.header, Bits: uint32(len(d.state.values)), CapacityWords: uint32((uint64(cap(d.state.values)) + 31) / 32)}
}
