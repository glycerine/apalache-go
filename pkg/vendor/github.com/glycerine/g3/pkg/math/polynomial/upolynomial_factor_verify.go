package polynomial

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Source: upolynomial.cpp:3074,3085,3100; util/debug.h:82-85. VERIFY does not
// consult assertions_enabled, and the GNU release false leaf is undefined.
func (m *UPolynomialManager) factorExactDivisionVerifyFailure(line int32) error {
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, m.modular.BorrowTraceTags()); e != nil {
		return e
	}
	defer borrow.unbind()
	expression := "exact_div(C, B, A)"
	switch line {
	case 3085:
		expression = "exact_div(A, D, C)"
	case 3100:
		expression = "exact_div(B, D, B)"
	}
	if e := diagnostic.BorrowProcessDebugContext().VerifyFailure("src/math/polynomial/upolynomial.cpp", line, expression); e != nil {
		return errors.Join(ErrUPolynomialAssertion, e)
	}
	return nil
}
