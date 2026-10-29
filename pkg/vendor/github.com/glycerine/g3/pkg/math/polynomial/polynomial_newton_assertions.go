package polynomial

// Source: src/math/polynomial/polynomial.cpp:2792,2822,2834,2841-2869,
// 2881-2886,2916,2941,2946.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// The predicate is evaluated only after the source assertion switch is read.
// This preserves erased/disabled assertions and closed debug-owner ordering.
func polynomialNewtonSourceAssert(line int32, expression string, predicate func() (bool, error)) error {
	return polynomialNewtonSourceAssertAt("src/math/polynomial/polynomial.cpp", line, expression, predicate)
}

func polynomialNewtonSourceAssertAt(file string, line int32, expression string, predicate func() (bool, error)) error {
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
	if e = diagnostic.DebugAssertionFailure(d, file, line, expression); e != nil {
		return errors.Join(ErrPolynomialAssertion, e)
	}
	return nil
}

func polynomialNewtonScalarAssert(line int32, expression string, condition bool) error {
	return polynomialNewtonSourceAssert(line, expression, func() (bool, error) { return condition, nil })
}
