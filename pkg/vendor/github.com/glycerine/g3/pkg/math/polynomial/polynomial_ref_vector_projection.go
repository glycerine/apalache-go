package polynomial

// Concrete retained projection/normalization consumers of
// src/util/ref_vector.h:90-94,128-132. Storage remains a native Go slice.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// PolynomialID is manager::id(polynomial const*) (polynomial.cpp:1689-1691).
// The projection membership query reads only the ID word, without reading the
// polynomial's reference count, size, monomials or coefficients.
func (m *MultivariatePolynomialCore) PolynomialID(p Polynomial) (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	flags, e := m.readWord(p, 4)
	return flags & 0x7fffffff, e
}

// Set increments incoming before reading/decrementing the old slot, including
// self-assignment. Interrupted ownership remains in the existing SET frame.
func (v *PolynomialRefVector) Set(i uint32, p Polynomial) error {
	return v.setWithReceipt(i, p, nil)
}

func (v *PolynomialRefVector) Shrink(n uint32) error {
	if e := v.open(); e != nil {
		return e
	}
	s := &v.state
	if s.setActive || s.draining || s.incoming != (Polynomial{}) {
		return ErrPolynomialPending
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil {
		return e
	}
	if enabled && uint64(n) > uint64(len(s.values)) {
		return diagnostic.DebugAssertionFailure(d, "src/util/ref_vector.h", 91, "sz <= m_nodes.size()")
	}
	if uint64(n) > uint64(len(s.values)) {
		return arena.ErrBounds
	}
	s.shrinking, s.draining = true, true
	s.shrinkTo, s.drain = n, n
	return s.finishShrink(v.owner)
}

func (s *polynomialReferenceVector) finishShrink(m *MultivariatePolynomialCore) (err error) {
	if !s.shrinking {
		return nil
	}
	if s.resetExecuting {
		return ErrPolynomialPending
	}
	s.resetExecuting = true
	defer func() {
		s.resetExecuting = false
		if err != nil {
			s.cleanupCause = err
		}
	}()
	for uint64(s.drain) < uint64(len(s.values)) {
		p, e := s.get(m, s.drain)
		if e != nil {
			return e
		}
		if e = polynomialReferenceDrop(m, &p, &s.drainDec); e != nil {
			return e
		}
		s.drain++
	}
	// Publish the new extent only after all forward tail DECs completed.
	s.values = s.values[:s.shrinkTo]
	s.drain = 0
	s.shrinking, s.draining = false, false
	s.cleanupCause = nil
	return nil
}

// CleanupRetained acknowledges only this vector's accepted SET, SHRINK or
// RESET. Borrowed polynomial/numerical callees must be acknowledged by their
// actual caller first. Unrelated vectors and manager work are untouched.
func (v *PolynomialRefVector) CleanupRetained() error {
	if v == nil || v.self != v {
		return ErrPolynomialOwner
	}
	if v.closed {
		return arena.ErrClosed
	}
	if v.state.setExecuting || v.state.resetExecuting {
		return ErrPolynomialPending
	}
	if e := v.state.abortSet(v.owner); e != nil {
		return e
	}
	if v.state.shrinking {
		return v.state.finishShrink(v.owner)
	}
	if v.state.draining {
		return v.state.resetMode(v.owner, true)
	}
	return nil
}
