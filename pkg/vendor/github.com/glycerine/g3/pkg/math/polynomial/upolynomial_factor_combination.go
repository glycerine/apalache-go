package polynomial

// Source: src/math/polynomial/upolynomial_factorization_int.h:124-417.
// Concrete ufactorization_combination_iterator, without template dispatch.

import (
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type upolyFactorCombinationState struct {
	factors          *UPolynomialFactors
	degrees          *upolyFactorDegreeState
	enabled          []bool
	current          []int32
	total, max, size int32
}

func (s *upolyFactorCombinationState) close() {
	s.current = nil
	s.enabled = nil
}
func (s *upolyFactorCombinationState) init(fs *UPolynomialFactors, ds *upolyFactorDegreeState) error {
	s.factors = fs
	s.degrees = ds
	s.total = int32(fs.count)
	s.max = int32(fs.count / 2)
	if e := factorHeaderAssert(fs.totalFactors > 1, 170, "factors.total_factors() > 1"); e != nil {
		return e
	}
	if e := factorHeaderAssert(fs.totalFactors == fs.count, 171, "factors.total_factors() == factors.distinct_factors()"); e != nil {
		return e
	}
	if fs.count > math.MaxInt32 {
		return arena.ErrBounds
	}
	if uint64(fs.count)+1 > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	if int(fs.count) > len(s.enabled) {
		s.enabled = append(s.enabled, make([]bool, int(fs.count)-len(s.enabled))...)
	} else {
		s.enabled = s.enabled[:fs.count]
	}
	for i := range s.enabled {
		s.enabled[i] = true
	}
	if int(fs.count)+1 > len(s.current) {
		s.current = append(s.current, make([]int32, int(fs.count)+1-len(s.current))...)
	} else {
		s.current = s.current[:int(fs.count)+1]
	}
	for i := range s.current {
		s.current[i] = int32(fs.count)
	}
	s.size = 0
	return nil
}
func (s *upolyFactorCombinationState) find(position, upper int32) (int32, error) {
	if position < 0 || uint32(position) >= uint32(len(s.current)) {
		return 0, arena.ErrBounds
	}
	candidate := s.current[position] + 1
	for candidate < upper {
		if candidate < 0 || uint32(candidate) >= uint32(len(s.enabled)) {
			return 0, arena.ErrBounds
		}
		if s.enabled[candidate] {
			break
		}
		candidate++
	}
	if candidate == upper {
		return -1, nil
	}
	return candidate, nil
}
func (s *upolyFactorCombinationState) degree() (uint32, error) {
	degree := uint32(0)
	for i := int32(0); i < s.size; i++ {
		idx := s.current[i]
		if idx < 0 || uint32(idx) >= s.factors.count {
			return 0, arena.ErrBounds
		}
		degree += upolyStateDegree(&s.factors.factors[idx])
	}
	return degree, nil
}
func (s *upolyFactorCombinationState) next(remove bool) (bool, error) {
	upper := int32(s.factors.count)
	for {
		idx := s.size - 1
		value := int32(-1)
		if remove {
			if e := factorHeaderAssert(s.size > 0, 202, "m_current_size > 0"); e != nil {
				return false, e
			}
			if s.size <= 0 {
				return false, arena.ErrBounds
			}
			for idx = s.size - 1; idx > 0; idx-- {
				chosen := s.current[idx]
				if chosen < 0 || uint32(chosen) >= uint32(len(s.enabled)) {
					return false, arena.ErrBounds
				}
				if e := factorHeaderAssert(s.enabled[chosen], 205, "m_enabled[m_current[current_i]]"); e != nil {
					return false, e
				}
				s.enabled[chosen] = false
				s.current[idx] = upper
			}
			chosen := s.current[0]
			if chosen < 0 || uint32(chosen) >= uint32(len(s.enabled)) {
				return false, arena.ErrBounds
			}
			if e := factorHeaderAssert(s.enabled[chosen], 210, "m_enabled[m_current[0]]"); e != nil {
				return false, e
			}
			s.enabled[chosen] = false
			remove = false
			s.total -= s.size
			s.max = s.total / 2
		}
		for idx >= 0 {
			if uint32(idx+1) >= uint32(len(s.current)) {
				return false, arena.ErrBounds
			}
			next, e := s.find(idx, s.current[idx+1])
			if e != nil {
				return false, e
			}
			value = next
			if value >= 0 {
				s.current[idx] = value
				break
			}
			idx--
		}
		for {
			if value == -1 {
				if s.size >= s.max {
					return false, nil
				}
				s.size++
				s.current[0] = -1
				idx = 0
				next, e := s.find(idx, upper)
				if e != nil {
					return false, e
				}
				value = next
				if value == -1 {
					return false, nil
				}
				s.current[idx] = value
			}
			for idx++; idx < s.size; idx++ {
				if uint32(idx) >= uint32(len(s.current)) {
					return false, arena.ErrBounds
				}
				s.current[idx] = s.current[idx-1]
				next, e := s.find(idx, upper)
				if e != nil {
					return false, e
				}
				value = next
				if value == -1 {
					s.current[0] = -1
					break
				}
				s.current[idx] = value
			}
			if value != -1 {
				break
			}
		}
		degree, e := s.degree()
		if e != nil {
			return false, e
		}
		included, e := s.degrees.get(degree)
		if e != nil {
			return false, e
		}
		if included {
			return true, nil
		}
	}
}
func (s *upolyFactorCombinationState) left(out *upolyVectorState) error {
	return s.leftWithSourceContext(out, nil, integerAllocationInvocationContext{})
}
func (s *upolyFactorCombinationState) leftWithSourceContext(out *upolyVectorState, child *unumeral.IntegerAddMulInvocationReceipt, context integerAllocationInvocationContext) error {
	m := s.factors.manager
	if e := factorHeaderAssert(s.size > 0, 347, "m_current_size > 0"); e != nil {
		return e
	}
	if s.size <= 0 {
		return arena.ErrBounds
	}
	idx := s.current[0]
	if idx < 0 || uint32(idx) >= s.factors.count {
		return arena.ErrBounds
	}
	if e := m.setViewWithSourceContext(out, m.stateView(&s.factors.factors[idx]), context); e != nil {
		return e
	}
	for i := int32(1); i < s.size; i++ {
		idx := s.current[i]
		if idx < 0 || uint32(idx) >= s.factors.count {
			return arena.ErrBounds
		}
		if e := m.mulWithSourceContext(m.stateView(out), m.stateView(&s.factors.factors[idx]), out, child, context); e != nil {
			return e
		}
	}
	return nil
}
func (s *upolyFactorCombinationState) leftTail(value, out *unumeral.Int) error {
	return s.leftTailWithSourceContext(value, out, integerAllocationInvocationContext{})
}
func (s *upolyFactorCombinationState) leftTailWithSourceContext(value, out *unumeral.Int, context integerAllocationInvocationContext) error {
	m := s.factors.manager
	if e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Copy(out, value); e != nil {
		return e
	}
	for i := int32(0); i < s.size; i++ {
		idx := s.current[i]
		if idx < 0 || uint32(idx) >= s.factors.count {
			return arena.ErrBounds
		}
		a, e := m.read(m.stateView(&s.factors.factors[idx]), 0)
		if e != nil {
			return e
		}
		if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Mul(out, a, out); e != nil {
			return e
		}
	}
	return nil
}
func (s *upolyFactorCombinationState) right(value, out *unumeral.Int, polynomial *upolyVectorState) error {
	return s.rightWithSourceContext(value, out, polynomial, nil, integerAllocationInvocationContext{})
}
func (s *upolyFactorCombinationState) rightWithSourceContext(value, out *unumeral.Int, polynomial *upolyVectorState, child *unumeral.IntegerAddMulInvocationReceipt, context integerAllocationInvocationContext) error {
	m := s.factors.manager
	if polynomial != nil {
		if e := factorHeaderAssert(s.size > 0, 389, "m_current_size > 0"); e != nil {
			return e
		}
		if e := m.reset(polynomial); e != nil {
			return e
		}
	} else {
		if e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Copy(out, value); e != nil {
			return e
		}
	}
	selection := uint32(0)
	for current := uint32(0); current < s.factors.count; {
		if current >= uint32(len(s.enabled)) {
			return arena.ErrBounds
		}
		if !s.enabled[current] {
			current++
			continue
		}
		if selection >= s.factors.count+1 || int32(current) < s.current[selection] {
			enabled, e := factorHeaderAssertionsEnabled()
			if e != nil {
				return e
			}
			if enabled {
				mult, e := s.factors.multiplicity(current)
				if e != nil {
					return e
				}
				if mult != 1 {
					line := int32(377)
					if polynomial != nil {
						line = 403
					}
					if e := factorHeaderAssertionFailure(line, "m_factors.get_degree(current) == 1"); e != nil {
						return e
					}
				}
			}
			fv := m.stateView(&s.factors.factors[current])
			if polynomial == nil {
				a, e := m.read(fv, 0)
				if e != nil {
					return e
				}
				if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Mul(out, a, out); e != nil {
					return e
				}
			} else if polynomial.size == 0 {
				if e := m.setViewWithSourceContext(polynomial, fv, context); e != nil {
					return e
				}
			} else {
				if e := m.mulWithSourceContext(m.stateView(polynomial), fv, polynomial, child, context); e != nil {
					return e
				}
			}
			current++
		} else {
			current++
			selection++
		}
	}
	return nil
}

type UPolynomialFactorCombination struct {
	self        *UPolynomialFactorCombination
	degreeOwner *UPolynomialDegreeSet
	state       upolyFactorCombinationState
	header      uint64
	closed      bool
}

func NewUPolynomialFactorCombination(fs *UPolynomialFactors, ds *UPolynomialDegreeSet) (*UPolynomialFactorCombination, error) {
	if e := fs.check(); e != nil {
		return nil, e
	}
	if e := ds.check(); e != nil {
		return nil, e
	}
	b := fs.manager.config.Integer.Budget()
	n := uint64(reflect.TypeOf(UPolynomialFactorCombination{}).Size())
	if e := b.Reserve(n); e != nil {
		return nil, e
	}
	it := &UPolynomialFactorCombination{header: n, degreeOwner: ds}
	it.self = it
	if e := it.state.init(fs, &ds.state); e != nil {
		it.state.close()
		b.Release(n)
		return nil, e
	}
	return it, nil
}
func (it *UPolynomialFactorCombination) check() error {
	if it == nil || it.self != it {
		return ErrUPolynomialOwner
	}
	if it.closed {
		return arena.ErrClosed
	}
	if e := it.state.factors.check(); e != nil {
		return e
	}
	return it.degreeOwner.check()
}
func (it *UPolynomialFactorCombination) Next(remove bool) (bool, error) {
	if e := it.check(); e != nil {
		return false, e
	}
	return it.state.next(remove)
}
func (it *UPolynomialFactorCombination) LeftSize() (uint32, error) {
	if e := it.check(); e != nil {
		return 0, e
	}
	return uint32(it.state.size), nil
}
func (it *UPolynomialFactorCombination) RightSize() (uint32, error) {
	if e := it.check(); e != nil {
		return 0, e
	}
	return uint32(it.state.total - it.state.size), nil
}
func (it *UPolynomialFactorCombination) CurrentDegree() (uint32, error) {
	if e := it.check(); e != nil {
		return 0, e
	}
	return it.state.degree()
}
func (it *UPolynomialFactorCombination) Left(out UPolynomial) error {
	if e := it.check(); e != nil {
		return e
	}
	d, e := it.state.factors.manager.descriptor(out)
	if e != nil {
		return e
	}
	return it.state.left(&d.state)
}
func (it *UPolynomialFactorCombination) Right(out UPolynomial) error {
	if e := it.check(); e != nil {
		return e
	}
	d, e := it.state.factors.manager.descriptor(out)
	if e != nil {
		return e
	}
	return it.state.right(nil, nil, &d.state)
}
func (it *UPolynomialFactorCombination) LeftTail(value, out *unumeral.Int) error {
	if e := it.check(); e != nil {
		return e
	}
	return it.state.leftTail(value, out)
}
func (it *UPolynomialFactorCombination) RightTail(value, out *unumeral.Int) error {
	if e := it.check(); e != nil {
		return e
	}
	return it.state.right(value, out, nil)
}
func (it *UPolynomialFactorCombination) Close() error {
	if it == nil || it.self != it {
		return ErrUPolynomialOwner
	}
	if it.closed {
		return arena.ErrClosed
	}
	it.state.close()
	it.state.factors.manager.config.Integer.Budget().Release(it.header)
	it.closed = true
	return nil
}
