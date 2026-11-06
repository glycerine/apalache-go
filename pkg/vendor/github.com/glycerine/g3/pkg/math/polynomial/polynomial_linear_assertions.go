package polynomial

// Source: linear_eq_solver.h SASSERT/DEBUG_CODE sites.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func linearSourceAssertionsEnabled() (bool, error) {
	if !polynomialSourceDebug {
		return false, nil
	}
	return diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
}
func linearSourceAssertionFailure(line int32, expression string) error {
	e := diagnostic.DebugAssertionFailure(diagnostic.BorrowProcessDebugContext(), "src/math/polynomial/linear_eq_solver.h", line, expression)
	if e != nil {
		return errors.Join(ErrPolynomialAssertion, e)
	}
	return nil
}
func linearSourceAssert(predicate bool, line int32, expression string) error {
	enabled, e := linearSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled && !predicate {
		return linearSourceAssertionFailure(line, expression)
	}
	return nil
}
