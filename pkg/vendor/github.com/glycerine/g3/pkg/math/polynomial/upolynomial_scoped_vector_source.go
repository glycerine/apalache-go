package polynomial

// Source: src/util/scoped_numeral_vector.h:29-40;
// src/math/polynomial/upolynomial_factorization.cpp:583-674.

import (
	"github.com/glycerine/g3/pkg/arena"
)

// The caller owns the actual lexical vector and its retained destructor cursor.
// This source-only engine never acknowledges an unpublished numeric allocation.
func upolynomialScopedVectorSourceDel(m *UPolynomialManager, s *upolyVectorState, cursor *uint32) error {
	if m == nil || m.self != m || s == nil || cursor == nil {
		return ErrUPolynomialOwner
	}
	if *cursor > s.size {
		return arena.ErrBounds
	}
	for *cursor < s.size {
		a, e := m.read(m.stateView(s), *cursor)
		if e != nil {
			return e
		}
		if _, e = upolynomialSourceNumeralDel(m.config.Integer, a); e != nil {
			return e
		}
		*cursor++
	}
	s.size = 0
	m.freeBacking(s)
	return nil
}
