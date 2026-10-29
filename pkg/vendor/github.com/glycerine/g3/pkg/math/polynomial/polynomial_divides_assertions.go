package polynomial

// Source: src/math/polynomial/polynomial.cpp:5159.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func polynomialDividesSourceAssert(predicate func() (bool, error)) error {
	if !polynomialSourceDebug {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil || !enabled {
		return e
	}
	ok, e := predicate()
	if e != nil || ok {
		return e
	}
	if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/polynomial.cpp", 5159, "!is_zero(q)"); e != nil {
		return errors.Join(ErrPolynomialAssertion, e)
	}
	return nil
}
