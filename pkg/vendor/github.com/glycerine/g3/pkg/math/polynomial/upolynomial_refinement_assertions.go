package polynomial

// Source: src/math/polynomial/upolynomial.cpp:2757-2814.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func refinementSourceAssertionsEnabled() (bool, error) {
	if !polynomialSourceDebug {
		return false, nil
	}
	return diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
}
func refinementSourceAssertionFailure(line int32, expression string) error {
	e := diagnostic.DebugAssertionFailure(diagnostic.BorrowProcessDebugContext(), "src/math/polynomial/upolynomial.cpp", line, expression)
	if e != nil {
		return errors.Join(ErrUPolynomialAssertion, e)
	}
	return nil
}
func refinementSourceAssert(predicate bool, line int32, expression string) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled && !predicate {
		return refinementSourceAssertionFailure(line, expression)
	}
	return nil
}

// The numerical operand belongs inside SASSERT: disabling or erasing the
// assertion must skip Horner evaluation, its allocations and its guards.
func (m *UPolynomialManager) refinementSourceSignAssert(v UPolynomialView, b *unumeral.BinaryRational, want int, line int32, expression string, receipt *upolyBinaryEvaluationInvocationReceipt) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	sign, e := m.evalSignBinaryWithNegScopeAndReceipt(v, b, nil, receipt)
	if e != nil {
		return e
	}
	if sign != want {
		return m.scalarDivisionAssertionFailure(line, expression)
	}
	return nil
}
