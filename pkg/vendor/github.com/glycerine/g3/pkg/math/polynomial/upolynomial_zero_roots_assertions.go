package polynomial

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Source: upolynomial.h:589 and upolynomial.cpp:1275. The header predicate
// reports its actual source file instead of borrowing the .cpp location.
func (m *UPolynomialManager) zeroRootsNonemptyAssertion(count uint32, header bool) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	if count != 0 {
		return nil
	}
	if !header {
		return m.scalarDivisionAssertionFailure(1275, "sz > 0")
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, m.modular.BorrowTraceTags()); e != nil {
		return e
	}
	defer borrow.unbind()
	if e := diagnostic.DebugAssertionFailure(diagnostic.BorrowProcessDebugContext(), "src/math/polynomial/upolynomial.h", 589, "sz > 0"); e != nil {
		return errors.Join(ErrUPolynomialAssertion, e)
	}
	return nil
}
