package polynomial

// Source: src/math/polynomial/polynomial.cpp:228,322-348,356-373.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func monomialSourceAssertionsEnabled() (bool, error) {
	if !polynomialSourceDebug {
		return false, nil
	}
	return diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
}

func (m *MonomialManager) sourceAssertionFailure(line int32, expression string) error {
	m.assertionRunning = true
	defer func() { m.assertionRunning = false }()
	if e := diagnostic.DebugAssertionFailure(diagnostic.BorrowProcessDebugContext(), "src/math/polynomial/polynomial.cpp", line, expression); e != nil {
		return errors.Join(ErrMonomialAssertion, e)
	}
	return nil
}

func (m *MonomialManager) powerSourceIndexAssert(a Monomial, i uint32) error {
	enabled, e := monomialSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	n, e := m.sizeCore(a)
	if e != nil || i < n {
		return e
	}
	return m.sourceAssertionFailure(228, "idx < size()")
}

func (m *MonomialManager) constructorOrderAssert(a Monomial, i uint32) error {
	enabled, e := monomialSourceAssertionsEnabled()
	if e != nil || !enabled || i == 0 {
		return e
	}
	current, e := m.varCore(a, i)
	if e != nil {
		return e
	}
	previous, e := m.varCore(a, i-1)
	if e != nil || current > previous {
		return e
	}
	return m.sourceAssertionFailure(204, "i == 0 || get_var(i) > get_var(i-1)")
}

func (m *MonomialManager) constructorDegreeAssert(a Monomial, i uint32) error {
	enabled, e := monomialSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	degree, e := m.degreeAtCore(a, i)
	if e != nil || degree > 0 {
		return e
	}
	return m.sourceAssertionFailure(205, "degree(i) > 0")
}

func (m *MonomialManager) inputValidAssert(in monomialInput, line int32, expression string) error {
	enabled, e := monomialSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	valid, e := in.valid()
	if e != nil || valid {
		return e
	}
	return m.sourceAssertionFailure(line, expression)
}

func (m *MonomialManager) multiplicationResultValidAssert(a Monomial) error {
	enabled, e := monomialSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	valid, e := m.IsValid(a)
	if e != nil || valid {
		return e
	}
	return m.sourceAssertionFailure(974, "r->is_valid()")
}

func (m *MonomialManager) multiplicationDegreeAssert(a Monomial, left, right monomialInput) error {
	enabled, e := monomialSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	total, e := m.degreeCore(a)
	if e != nil {
		return e
	}
	dx, e := left.degreeSum()
	if e != nil {
		return e
	}
	dy, e := right.degreeSum()
	if e != nil || total == dx+dy {
		return e
	}
	return m.sourceAssertionFailure(975, "r->total_degree() == power_product_total_degree(sz1, pws1) + power_product_total_degree(sz2, pws2)")
}

func (m *MonomialManager) factoryContainsAssert(k monomialKey) error {
	enabled, e := monomialSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	_, found, e := m.table.find(m, k)
	if e != nil || found {
		return e
	}
	return m.sourceAssertionFailure(861, "m_monomials.contains(r)")
}

func (m *MonomialManager) factoryFindAssert(k monomialKey) error {
	enabled, e := monomialSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	pos, found, e := m.table.find(m, k)
	if e != nil {
		return e
	}
	if !found {
		if e = m.sourceAssertionFailure(862, "*(m_monomials.find_core(r)) == r"); e != nil {
			return e
		}
		return ErrMonomialPending
	}
	got, e := m.table.key(pos)
	if e != nil || got == k {
		return e
	}
	return m.sourceAssertionFailure(862, "*(m_monomials.find_core(r)) == r")
}

// The condition uses only already-loaded scalar locals. Assertions containing
// source object reads use concrete helpers below, after the runtime switch.
func (m *MonomialManager) sourceScalarAssert(line int32, expression string, condition bool) error {
	enabled, e := monomialSourceAssertionsEnabled()
	if e != nil || !enabled || condition {
		return e
	}
	return m.sourceAssertionFailure(line, expression)
}

func (m *MonomialManager) searchVariableAssert(a Monomial, i uint32, x uint32, variableFirst bool, line int32, expression string) error {
	enabled, e := monomialSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	v, e := m.varCore(a, i)
	if e != nil {
		return e
	}
	if variableFirst && v < x || !variableFirst && x < v {
		return nil
	}
	return m.sourceAssertionFailure(line, expression)
}

func (m *MonomialManager) searchWindowAssert(a Monomial, low, high, mid int32, midpoint bool) error {
	enabled, e := monomialSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	line, expression := int32(326), "0 <= low && high < static_cast<int>(m_size)"
	valid := low >= 0
	if midpoint {
		line, expression = 340, "low < mid && mid < high && high < static_cast<int>(m_size)"
		valid = low < mid && mid < high
	}
	if valid {
		n, e := m.sizeCore(a)
		if e != nil {
			return e
		}
		valid = high < int32(n)
	}
	if !valid {
		return m.sourceAssertionFailure(line, expression)
	}
	return nil
}

func (m *MonomialManager) searchPreviousAssert(a Monomial, mid, x uint32) error {
	enabled, e := monomialSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	previous, e := m.varCore(a, mid-1)
	if e != nil {
		return e
	}
	valid := previous < x
	if valid {
		current, e := m.varCore(a, mid)
		if e != nil {
			return e
		}
		valid = x == current
	}
	if !valid {
		return m.sourceAssertionFailure(341, "get_var(mid-1) < x && x == get_var(mid)")
	}
	return nil
}

func (m *MonomialManager) searchResultAssert(a Monomial, i, y uint32) error {
	enabled, e := monomialSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	valid := y != ^uint32(0)
	if valid {
		v, e := m.varCore(a, i)
		if e != nil {
			return e
		}
		valid = v <= y
	}
	if !valid {
		return m.sourceAssertionFailure(368, "y != null_var && get_var(i) <= y")
	}
	return nil
}

// Source: polynomial.cpp:1289. The extra uniqueness lookup belongs entirely
// to SASSERT; insertion performs its own independent lookup afterwards.
func (m *MonomialManager) renameUniqueAssert(t *monomialTable, k monomialKey) error {
	enabled, e := monomialSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	_, found, e := t.find(m, k)
	if e != nil || !found {
		return e
	}
	return m.sourceAssertionFailure(1289, "!new_table.contains(m)")
}
