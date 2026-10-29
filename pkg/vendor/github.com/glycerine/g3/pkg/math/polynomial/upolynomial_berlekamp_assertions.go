package polynomial

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Project SASSERT sites in this source unit use the actual debugger action.
func factorizationSourceAssertionFailure(line int32, expression string) error {
	if e := diagnostic.DebugAssertionFailure(diagnostic.BorrowProcessDebugContext(), "src/math/polynomial/upolynomial_factorization.cpp", line, expression); e != nil {
		return errors.Join(ErrUPolynomialAssertion, e)
	}
	return nil
}

// The assertion dialog and the reached source body borrow this actual matrix.
func (q *upolyBerlekampMatrix) beginAssertionBorrow() (bool, error) {
	if q.displaying {
		return false, nil
	}
	q.displaying = true
	if e := q.traceBorrow.bindManagers([3]*UPolynomialManager{q.m}, q.m.modular.BorrowTraceTags()); e != nil {
		q.displaying = false
		return false, e
	}
	return true, nil
}
func (q *upolyBerlekampMatrix) endAssertionBorrow(owned bool) {
	if owned {
		q.traceBorrow.unbind()
		q.displaying = false
	}
}

func (m *UPolynomialManager) factorizationAssertionFailure(line int32, expression string) error {
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, m.modular.BorrowTraceTags()); e != nil {
		return e
	}
	defer borrow.unbind()
	return factorizationSourceAssertionFailure(line, expression)
}
