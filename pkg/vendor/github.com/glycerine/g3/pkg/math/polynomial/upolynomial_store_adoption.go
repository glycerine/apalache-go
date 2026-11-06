package polynomial

// Concrete cold owner binding for actual cross-core coefficient ownership.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

type upolyStoreBinding struct {
	origin uint64
	store  *UPolynomialCoefficientStore
}

func (m *UPolynomialManager) boundStore(origin uint64) *UPolynomialCoefficientStore {
	if origin == m.store.origin {
		return m.store
	}
	for _, b := range m.bindings {
		if b.origin == origin {
			return b.store
		}
	}
	return nil
}
func (m *UPolynomialManager) stateOrigin(s *upolyVectorState) uint64 {
	if s.origin == 0 {
		return m.store.origin
	}
	return s.origin
}
func (m *UPolynomialManager) stateStore(s *upolyVectorState) *UPolynomialCoefficientStore {
	return m.boundStore(m.stateOrigin(s))
}

// BorrowCoefficientStore provides explicit cold provenance for source raw
// pointers. It retains a distinct foreign store until core Close. Bindings use
// a native slice; this method copies no coefficients or arithmetic state.
func (m *UPolynomialManager) BorrowCoefficientStore(store *UPolynomialCoefficientStore) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := store.check(); e != nil {
		return e
	}
	if store.integer != m.config.Integer {
		return ErrUPolynomialOwner
	} // actual shared MPZ backend required cross-Integer
	if m.boundStore(store.origin) != nil {
		return nil
	}
	if store.owners == math.MaxUint32 {
		return arena.ErrSlotLimit
	}
	store.owners++
	m.bindings = append(m.bindings, upolyStoreBinding{store.origin, store})
	return nil
}
func (m *UPolynomialManager) adoptState(source *UPolynomialManager, s *upolyVectorState) error {
	if s.backing == 0 {
		return nil
	}
	store := source.stateStore(s)
	if store == nil {
		return ErrUPolynomialOwner
	}
	return m.BorrowCoefficientStore(store)
}

// SwapFrom is the concrete two-owner source vector swap. Binding preparations
// finish before the infallible scalar swap. Native directory growth does not
// allocate through the numeric Runtime or introduce a container fault prefix.
func (m *UPolynomialManager) SwapFrom(dst UPolynomial, source *UPolynomialManager, src UPolynomial) error {
	a, e := m.descriptor(dst)
	if e != nil {
		return e
	}
	b, e := source.descriptor(src)
	if e != nil {
		return e
	}
	if e = m.adoptState(source, &b.state); e != nil {
		return e
	}
	if e = source.adoptState(m, &a.state); e != nil {
		return e
	}
	a.state, b.state = b.state, a.state
	return nil
}
