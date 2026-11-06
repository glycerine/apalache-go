package polynomial

// Source: src/math/polynomial/polynomial.cpp:4567,4596.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func polynomialSquareFreeSourceAssert(line int32, predicate func() (bool, error)) error {
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
	if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/polynomial.cpp", line, "!eq(p, r) || p == r.get()"); e != nil {
		return errors.Join(ErrPolynomialAssertion, e)
	}
	return nil
}
