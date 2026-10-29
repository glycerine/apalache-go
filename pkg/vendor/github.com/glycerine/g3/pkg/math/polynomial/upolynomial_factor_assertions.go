package polynomial

// Source: src/math/polynomial/upolynomial_factorization_int.h SASSERT sites.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func factorHeaderAssertionsEnabled() (bool, error) {
	if !polynomialSourceDebug {
		return false, nil
	}
	return diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
}
func factorHeaderAssertionFailure(line int32, expression string) error {
	e := diagnostic.DebugAssertionFailure(diagnostic.BorrowProcessDebugContext(), "src/math/polynomial/upolynomial_factorization_int.h", line, expression)
	if e != nil {
		return errors.Join(ErrUPolynomialAssertion, e)
	}
	return nil
}
func factorHeaderAssert(predicate bool, line int32, expression string) error {
	enabled, e := factorHeaderAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled && !predicate {
		return factorHeaderAssertionFailure(line, expression)
	}
	return nil
}
