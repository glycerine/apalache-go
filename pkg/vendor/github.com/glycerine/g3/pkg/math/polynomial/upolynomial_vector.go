package polynomial

// Concrete _scoped_numeral_vector<mpzzp_manager> scalar ownership methods.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *UPolynomialManager) shrink(s *upolyVectorState, n uint32) error {
	old := s.size
	if old == n {
		return nil
	}
	// Source callbacks run before base-vector shrink and its debug assertion.
	for i := n; i < old; i++ {
		if e := m.modular.Del(&m.values(s)[i]); e != nil {
			return e
		}
	}
	if polynomialSourceDebug && n > old {
		return ErrUPolynomialAssertion
	}
	if s.backing == 0 {
		return nil
	}
	if uint64(n) > uint64(len(m.values(s))) {
		return arena.ErrBounds
	}
	s.size = n
	return nil
}

// Shrink is the owning scoped-vector action, without core set_size's trim.
func (m *UPolynomialManager) Shrink(p UPolynomial, n uint32) error {
	d, e := m.descriptor(p)
	if e != nil {
		return e
	}
	return m.shrink(&d.state, n)
}
func (m *UPolynomialManager) Resize(p UPolynomial, n uint32) error {
	d, e := m.descriptor(p)
	if e != nil {
		return e
	}
	if n <= d.state.size {
		if e = m.shrink(&d.state, n); e != nil {
			return e
		}
	}
	return m.reserve(&d.state, n)
}
func (m *UPolynomialManager) PushCoefficient(p UPolynomial, n *unumeral.Int) error {
	d, e := m.descriptor(p)
	if e != nil {
		return e
	}
	s := &d.state
	if s.size == math.MaxUint32 {
		return arena.ErrBounds
	}
	i := s.size
	if e = m.reserve(s, i+1); e != nil {
		return e
	}
	return m.modular.Copy(&m.values(s)[i], n)
}
func (m *UPolynomialManager) PopCoefficient(p UPolynomial) error {
	d, e := m.descriptor(p)
	if e != nil {
		return e
	}
	return m.shrink(&d.state, d.state.size-1)
}
