package polynomial

// Source: src/math/polynomial/polynomial.cpp; src/math/polynomial/polynomial.h.

import (
	"math"
)

func (m *MonomialManager) IndexOf(a Monomial, x uint32) (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	n, e := m.sizeCore(a)
	if e != nil {
		return 0, e
	}
	return m.indexCore(a, n, x)
}
func (m *MonomialManager) indexCore(a Monomial, n, x uint32) (uint32, error) {
	if n == 0 {
		return math.MaxUint32, nil
	}
	last := n - 1
	v, e := m.varCore(a, last)
	if e != nil {
		return 0, e
	}
	if v == x {
		return last, nil
	}
	if n < 8 {
		for i := last; i > 0; {
			i--
			v, e = m.varCore(a, i)
			if e != nil {
				return 0, e
			}
			if v == x {
				return i, nil
			}
		}
		return math.MaxUint32, nil
	}
	low, high := int32(0), int32(last)
	for {
		mid := low + (high-low)/2
		v, e = m.varCore(a, uint32(mid))
		if e != nil {
			return 0, e
		}
		if x > v {
			low = mid + 1
		} else if x < v {
			high = mid - 1
		} else {
			return uint32(mid), nil
		}
		if low > high {
			return math.MaxUint32, nil
		}
	}
}
func (m *MonomialManager) DegreeOf(a Monomial, x uint32) (uint32, error) {
	i, e := m.IndexOf(a, x)
	if e != nil {
		return 0, e
	}
	if i == math.MaxUint32 {
		return 0, nil
	}
	return m.degreeAtCore(a, i)
}
func (m *MonomialManager) MaxVariable(a Monomial) (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	n, e := m.sizeCore(a)
	if e != nil {
		return 0, e
	}
	if n == 0 {
		return math.MaxUint32, nil
	}
	return m.varCore(a, n-1)
}
func (m *MonomialManager) MaxVariableDegree(a Monomial) (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	n, e := m.sizeCore(a)
	if e != nil || n == 0 {
		return 0, e
	}
	return m.degreeAtCore(a, n-1)
}
func (m *MonomialManager) maxSmallerCore(a Monomial, x uint32) (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	n, e := m.sizeCore(a)
	if e != nil {
		return 0, e
	}
	if n == 0 {
		return math.MaxUint32, nil
	}
	if n < 8 {
		for i := n; i > 0; {
			i--
			v, e := m.varCore(a, i)
			if e != nil {
				return 0, e
			}
			if v < x {
				return m.varCore(a, i)
			}
		}
		return math.MaxUint32, nil
	}
	low, high := int32(0), int32(n-1)
	v, e := m.varCore(a, uint32(low))
	if e != nil {
		return 0, e
	}
	if x <= v {
		return math.MaxUint32, nil
	}
	v, e = m.varCore(a, uint32(high))
	if e != nil {
		return 0, e
	}
	if x > v {
		return m.varCore(a, uint32(high))
	}
	v, e = m.varCore(a, uint32(high))
	if e != nil {
		return 0, e
	}
	if x == v {
		if e := m.sourceScalarAssert(322, "high > 0", high > 0); e != nil {
			return 0, e
		}
		return m.varCore(a, uint32(high-1))
	}
	for {
		if e := m.searchWindowAssert(a, low, high, 0, false); e != nil {
			return 0, e
		}
		if e := m.searchVariableAssert(a, uint32(low), x, true, 327, "get_var(low) < x"); e != nil {
			return 0, e
		}
		if e := m.searchVariableAssert(a, uint32(high), x, false, 328, "x < get_var(high)"); e != nil {
			return 0, e
		}
		if e := m.sourceScalarAssert(329, "low < high", low < high); e != nil {
			return 0, e
		}
		if high == low+1 {
			if e := m.searchVariableAssert(a, uint32(low), x, true, 331, "get_var(low) < x"); e != nil {
				return 0, e
			}
			if e := m.searchVariableAssert(a, uint32(low+1), x, false, 332, "x < get_var(low+1)"); e != nil {
				return 0, e
			}
			return m.varCore(a, uint32(low))
		}
		if e := m.sourceScalarAssert(335, "high > low + 1", high > low+1); e != nil {
			return 0, e
		}
		mid := low + (high-low)/2
		if e := m.sourceScalarAssert(337, "low < mid && mid < high", low < mid && mid < high); e != nil {
			return 0, e
		}
		v, e = m.varCore(a, uint32(mid))
		if e != nil {
			return 0, e
		}
		if v == x {
			if e := m.searchWindowAssert(a, low, high, mid, true); e != nil {
				return 0, e
			}
			if e := m.searchPreviousAssert(a, uint32(mid), x); e != nil {
				return 0, e
			}
			return m.varCore(a, uint32(mid-1))
		}
		if x < v {
			high = mid
		} else {
			if e := m.sourceScalarAssert(348, "x_mid < x", v < x); e != nil {
				return 0, e
			}
			low = mid
		}
	}
}

func (m *MonomialManager) IsValid(a Monomial) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	n, e := m.sizeCore(a)
	if e != nil {
		return false, e
	}
	valid, e := m.validPowerProductCore(a, n)
	if e != nil || !valid {
		return false, e
	}
	n, e = m.sizeCore(a)
	if e != nil {
		return false, e
	}
	total, e := m.degreeSumCore(a, n)
	if e != nil {
		return false, e
	}
	stored, e := m.degreeCore(a)
	return total == stored, e
}
func (m *MonomialManager) IsSquare(a Monomial) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	n, e := m.sizeCore(a)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < n; i++ {
		d, e := m.degreeAtCore(a, i)
		if e != nil {
			return false, e
		}
		if d%2 != 0 {
			return false, nil
		}
	}
	return true, nil
}
func (m *MonomialManager) IsPowerOfTwo(a Monomial) (bool, error) { return m.IsSquare(a) }

func (m *MonomialManager) sizeCore(a Monomial) (uint32, error) {
	l, e := m.location(a)
	if e != nil {
		return 0, e
	}
	return m.allocator.ReadUint32(l.raw, 12)
}
func (m *MonomialManager) degreeCore(a Monomial) (uint32, error) {
	l, e := m.location(a)
	if e != nil {
		return 0, e
	}
	return m.allocator.ReadUint32(l.raw, 8)
}
func (m *MonomialManager) varCore(a Monomial, i uint32) (uint32, error) {
	l, e := m.location(a)
	if e != nil {
		return 0, e
	}
	if e := m.powerSourceIndexAssert(a, i); e != nil {
		return 0, e
	}
	return m.allocator.ReadUint32(l.raw, 20+uint64(i)*8)
}

// degreeAtCore implements degree(idx): source get_power forms its reference
// (and DEBUG index assertion) before reading only the degree member.
func (m *MonomialManager) degreeAtCore(a Monomial, i uint32) (uint32, error) {
	l, e := m.location(a)
	if e != nil {
		return 0, e
	}
	if e := m.powerSourceIndexAssert(a, i); e != nil {
		return 0, e
	}
	return m.allocator.ReadUint32(l.raw, 24+uint64(i)*8)
}
func (m *MonomialManager) sizeInfoCore(a Monomial) (MonomialInfo, error) {
	n, e := m.sizeCore(a)
	return MonomialInfo{Size: n}, e
}

// The free power-product helpers read raw array fields, not get_power's
// debug index assertion or unrelated fields of each power.
func (m *MonomialManager) powerFieldCore(a Monomial, i uint32, field uint64) (uint32, error) {
	l, e := m.location(a)
	if e != nil {
		return 0, e
	}
	return m.allocator.ReadUint32(l.raw, 20+uint64(i)*8+field)
}
func (m *MonomialManager) validPowerProductCore(a Monomial, n uint32) (bool, error) {
	for i := uint32(1); i < n; i++ {
		previous, e := m.powerFieldCore(a, i-1, 0)
		if e != nil {
			return false, e
		}
		current, e := m.powerFieldCore(a, i, 0)
		if e != nil {
			return false, e
		}
		if previous >= current {
			return false, nil
		}
	}
	return true, nil
}
func (m *MonomialManager) degreeSumCore(a Monomial, n uint32) (uint32, error) {
	var sum uint32
	for i := uint32(0); i < n; i++ {
		v, e := m.powerFieldCore(a, i, 4)
		if e != nil {
			return 0, e
		}
		sum += v
	}
	return sum, nil
}
func (m *MonomialManager) degreeInfoCore(a Monomial) (MonomialInfo, error) {
	n, e := m.degreeCore(a)
	return MonomialInfo{TotalDegree: n}, e
}

func (m *MonomialManager) MaxSmallerThan(a Monomial, x uint32) (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	if e := m.sourceScalarAssert(356, "x != null_var", x != math.MaxUint32); e != nil {
		return 0, e
	}
	v, e := m.maxSmallerCore(a, x)
	if e != nil {
		return 0, e
	}
	if polynomialSourceDebug {
		found := false
		for i := uint32(0); ; i++ {
			n, e := m.sizeCore(a)
			if e != nil {
				return 0, e
			}
			if i >= n {
				break
			}
			q, e := m.varCore(a, i)
			if e != nil {
				return 0, e
			}
			if q < x {
				if e := m.tracePredecessorResult(a, x, v, i); e != nil {
					return 0, e
				}
				if e := m.searchResultAssert(a, i, v); e != nil {
					return 0, e
				}
			}
			q, e = m.varCore(a, i)
			if e != nil {
				return 0, e
			}
			if q == v {
				found = true
			}
		}
		if e := m.sourceScalarAssert(373, "y == null_var || (y < x && found)", v == math.MaxUint32 || (v < x && found)); e != nil {
			return 0, e
		}
	}
	return v, nil
}
