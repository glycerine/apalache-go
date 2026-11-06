package polynomial

// Source: src/math/polynomial/polynomial.cpp:2489.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *MultivariatePolynomialCore) directorySlotSourceAssert(id uint32) error {
	if !polynomialSourceDebug {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil || !enabled {
		return e
	}
	if uint64(id) >= uint64(len(m.polynomials)) {
		return arena.ErrBounds
	}
	if m.polynomials[id] == (Polynomial{}) {
		return nil
	}
	if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/polynomial.cpp", 2489, "m_polynomials[id] == 0"); e != nil {
		return errors.Join(ErrPolynomialAssertion, e)
	}
	return nil
}
