package polynomial

// Source: src/math/polynomial/polynomial.cpp:2722,2758,5941.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func polynomialArithmeticSourceAssert(line int32, predicate func() (bool, error)) error {
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
	if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/polynomial.cpp", line, "m_cheap_som_buffer.empty()"); e != nil {
		return errors.Join(ErrPolynomialAssertion, e)
	}
	return nil
}
